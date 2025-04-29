package docker

import (
    "fmt"
    "context"
    "io"
    "log"
    // "os"
    "sync"
    "sync/atomic"
    "time"
    "errors"

    "github.com/Aadithya-J/alcaIDE/model"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/image"
    "github.com/docker/docker/client"
)

const (
    ContainerStopTimeout    = 5 * time.Second
    ContainerCleanupTimeout = 10 * time.Second
)

type DockerManager struct {
    cli               *client.Client
    languageImages    map[string]string 
    availablePools    map[string]chan *model.ContainerInfo
    allContainers     map[string]*model.ContainerInfo
    allContainersLock sync.RWMutex
    poolsLock         sync.RWMutex
    shuttingDown      atomic.Bool
}

func NewManager(langImages map[string]string) (*DockerManager, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return nil, err
    }

    if len(langImages) == 0 {
        return nil, fmt.Errorf("No language images provided")
    }

    return &DockerManager{
        cli:            cli,
        languageImages: langImages,
        availablePools: make(map[string]chan *model.ContainerInfo),
        allContainers:  make(map[string]*model.ContainerInfo),
    }, nil
}

func (m *DockerManager) PullImages(ctx context.Context) error {
    var wg sync.WaitGroup
    errChan := make(chan error, len(m.languageImages))

    log.Printf("Pulling Docker images: %s...", m.languageImages)

    for lang, imageName := range m.languageImages {
        wg.Add(1)
        go func(l, img string) {
            defer wg.Done()
            log.Printf("Pulling image for %s: %s...", l, img)

            reader, err := m.cli.ImagePull(ctx, img, image.PullOptions{})
            if err != nil {
                log.Printf("Failed to pull Docker image %s: %v", img, err)
                errChan <- fmt.Errorf("failed to pull image %s: %w", img, err)
                return
            }
            defer reader.Close()

            _, err = io.Copy(io.Discard, reader)
            if err != nil {
                log.Printf("Warning: Failed to copy image pull output for %s: %v", img, err)
            }
            log.Printf("Image %s pulled successfully or already exists.", img)
        }(lang, imageName)
    }
    wg.Wait()
    close(errChan)

    var pullErrors []error
    for err := range errChan {
        pullErrors = append(pullErrors, err)
    }

    if len(pullErrors) > 0 {
        log.Printf("Warning: Some images failed to pull: %v", pullErrors)
    } else {
        log.Println("All images pulled successfully.")
    }
    return nil
}

func (m *DockerManager) StartInitialContainers(ctx context.Context, countPerLang int) error {
    if countPerLang <= 0 {
        return fmt.Errorf("invalid number of containers per language: %d", countPerLang)
    }

    log.Printf("Creating and starting %d initial containers *per language*...", countPerLang)

    m.poolsLock.Lock()
    for lang := range m.languageImages {
        m.availablePools[lang] = make(chan *model.ContainerInfo, countPerLang)
    }
    m.poolsLock.Unlock()

    var wg sync.WaitGroup
    errChan := make(chan error, len(m.languageImages)*countPerLang)
    totalContainersToStart := 0

    for lang, imageName := range m.languageImages { 
        log.Printf("starting ocontainers for %s using image %s", lang, imageName)
        for i := 0; i < countPerLang; i++ {
            totalContainersToStart++
            wg.Add(1)
            go func(lang, imageName string, containerIndex int) {

                defer wg.Done()
                log.Printf("Creating container %d for %s...", containerIndex, lang)

                resp, err := m.cli.ContainerCreate(ctx, &container.Config{
                    Image: imageName,
                    Cmd:   []string{"sleep", "infinity"},
                    Tty:   false,
                }, nil, nil, nil, "")
                if err != nil {
                    errChan <- fmt.Errorf("failed to create container %d for %s: %w", containerIndex, lang, err)
                    return
                }
                containerID := resp.ID
                err = m.cli.ContainerStart(ctx, containerID, container.StartOptions{})
                if err != nil {
                    errChan <- fmt.Errorf("failed to start container %d for %s: %w", containerIndex, lang, err)
                    rmCtx, rmCancel := context.WithTimeout(context.Background(), ContainerCleanupTimeout)
                    defer rmCancel()
                    rmErr := m.cli.ContainerRemove(rmCtx, containerID, container.RemoveOptions{Force: true})
                    if rmErr != nil {
                        log.Printf("Warning: Failed to remove unstartable container %s: %v", containerID, rmErr)
                    }
                    return 
                }

                log.Printf("Started container %d for %s: %s", containerIndex+1, lang, containerID)

                containInfo := &model.ContainerInfo{ID: containerID}

                m.allContainersLock.Lock()
                m.allContainers[containerID] = containInfo
                m.allContainersLock.Unlock()

                m.poolsLock.Lock()
                poolsChan, ok := m.availablePools[lang]
                m.poolsLock.Unlock()
                if ok {
                    poolsChan <- containInfo
                    log.Printf("Container %d for %s added to available pool: %s", containerIndex+1, lang, containerID)
                } else {
                    log.Printf("Warning: No pool found for language %s. Container %d not added to pool.", lang, containerIndex)
                }
            }(lang, imageName, i)
        }
    }
    wg.Wait()
    close(errChan)
    var startupErrors []error
    for err := range errChan {
        log.Printf("Error during container startup: %v", err)
        startupErrors = append(startupErrors, err)
    }

    m.allContainersLock.RLock()
    numStarted := len(m.allContainers)
    m.allContainersLock.RUnlock()
    if numStarted == 0 {
        log.Fatal("Fatal: No containers were started successfully.")
    }
    if len(startupErrors) > 0 {
        log.Printf("Warning: Some containers failed to start: %v", startupErrors)
    }

    log.Printf("%d containers started successfully.", numStarted)
    return nil
}

func (m *DockerManager) AcquireContainer(ctx context.Context, language string) (*model.ContainerInfo, error) {
    m.poolsLock.RLock()
    poolChan, ok := m.availablePools[language]
    m.poolsLock.RUnlock()

    if !ok {
        return nil, fmt.Errorf("no container pool available for language: %s", language)
    }

    log.Printf("Attempting to acquire container for %s...", language)
    select {
    case container := <-poolChan:
        log.Printf("Container %s acquired for %s.", container.ID, language)
        return container, nil
    case <-ctx.Done():
        log.Printf("Context cancelled while waiting for %s container: %v", language, ctx.Err())
        return nil, fmt.Errorf("failed to acquire %s container: %w", language, ctx.Err())
    }
}

func (m *DockerManager) ReleaseContainer(container *model.ContainerInfo, language string) {
    if container == nil {
        log.Println("Warning: Attempted to release a nil container.")
        return
    }

    if m.shuttingDown.Load() {
        log.Printf("Shutdown in progress. Not returning container %s (%s) to pool.", container.ID, language)
        return
    }

    m.poolsLock.RLock()
    poolChan, ok := m.availablePools[language]
    m.poolsLock.RUnlock()

    if !ok {
        log.Printf("Warning: No pool found for language %s to release container %s.", language, container.ID)
        return
    }

    log.Printf("Releasing container %s for %s", container.ID, language)
    select {
    case poolChan <- container:
        log.Printf("Container %s returned to %s pool.", container.ID, language)
    default:
        log.Printf("Warning: Could not return container %s to %s pool (pool might be full or closed).", container.ID, language)
    }
}

func (m *DockerManager) GetContainers() []*model.ContainerInfo {
    m.allContainersLock.RLock()
    defer m.allContainersLock.RUnlock()

    listCopy := make([]*model.ContainerInfo, 0, len(m.allContainers))
    for _, c := range m.allContainers {
        listCopy = append(listCopy, c)
    }
    return listCopy
}


func (m *DockerManager) CleanupContainers() {
    m.shuttingDown.Store(true)

    m.poolsLock.Lock()
    log.Println("Closing all language container pools...")
    for lang, poolChan := range m.availablePools {
        close(poolChan)
        log.Printf("Closed pool for %s.", lang)
    }

    m.availablePools = make(map[string]chan *model.ContainerInfo)
    m.poolsLock.Unlock()

    m.allContainersLock.Lock()
    defer m.allContainersLock.Unlock()

    if len(m.allContainers) == 0 {
        log.Println("No containers managed by this manager to clean up.")
        return
    }

    log.Printf("Stopping and removing %d managed containers...", len(m.allContainers))

    cleanupTimeout := ContainerCleanupTimeout + (time.Second * time.Duration(len(m.allContainers)))
    cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), cleanupTimeout)
    defer cancelCleanup()

    var wg sync.WaitGroup
    for id, c := range m.allContainers {
        wg.Add(1)
        go func(containerID string, contInfo *model.ContainerInfo) {
            defer wg.Done()
            log.Printf("Stopping container %s...", containerID)

            stopTimeoutSecs := int(ContainerStopTimeout.Seconds())
            stopOpts := container.StopOptions{Timeout: &stopTimeoutSecs}

            if err := m.cli.ContainerStop(cleanupCtx, containerID, stopOpts); err != nil {
                if errors.Is(cleanupCtx.Err(), context.DeadlineExceeded) {
                    log.Printf("Context deadline exceeded before stopping container %s.", containerID)
                } else if cleanupCtx.Err() != nil {
                    log.Printf("Context cancelled before stopping container %s: %v", containerID, cleanupCtx.Err())
                } else {
                    log.Printf("Error stopping container %s: %v. Attempting removal anyway.", containerID, err)
                }
            } else {
                log.Printf("Container %s stopped.", containerID)
            }

            if cleanupCtx.Err() != nil {
                log.Printf("Context done before removing container %s.", containerID)
                return
            }
            log.Printf("Removing container %s...", containerID)
            removeOpts := container.RemoveOptions{Force: true} // Force remove if stop failed/timed out
            if err := m.cli.ContainerRemove(cleanupCtx, containerID, removeOpts); err != nil {
                if errors.Is(cleanupCtx.Err(), context.DeadlineExceeded) {
                    log.Printf("Context deadline exceeded during removal of container %s.", containerID)
                } else if cleanupCtx.Err() != nil {
                    log.Printf("Context cancelled during removal of container %s: %v", containerID, cleanupCtx.Err())
                } else {
                    log.Printf("Error removing container %s: %v", containerID, err)
                }
            } else {
                log.Printf("Container %s removed.", containerID)
            }
        }(id, c)
    }
    wg.Wait()

    if cleanupCtx.Err() == context.DeadlineExceeded {
        log.Println("Warning: Container cleanup timed out.")
    } else {
        log.Println("Finished container cleanup.")
    }
    m.allContainers = make(map[string]*model.ContainerInfo)
}

func (m *DockerManager) Close() {
    log.Println("Closing Docker Client...")
    if err := m.cli.Close(); err != nil {
        log.Printf("Error closing Docker client: %v", err)
    } else {
        log.Println("Docker client closed successfully.")
    }
}

func (m *DockerManager) GetClient() *client.Client {
    return m.cli
}