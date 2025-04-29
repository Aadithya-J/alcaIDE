package docker

import (
    "fmt"
    "context"
    "io"
    "log"
    "os"
    "sync"
    "time"

    "github.com/Aadithya-J/alcaIDE/model"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/image"
    "github.com/docker/docker/client"
)

const (
    DefaultImage            = "docker.io/library/python:3.11-slim"
    ContainerStopTimeout    = 5 * time.Second
    ContainerCleanupTimeout = 10 * time.Second
)

type DockerManager struct {
    cli        *client.Client
    containers []*model.ContainerInfo
    imageName  string
    availablePool chan *model.ContainerInfo
    containerMap map[string]*model.ContainerInfo
    containerMapMutex sync.RWMutex
}

func NewManager(imageName string) (*DockerManager, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return nil, err
    }
    if imageName == "" {
        imageName = DefaultImage
    }
    return &DockerManager{
        cli:        cli,
        containers: []*model.ContainerInfo{},
        imageName:  imageName,
        containerMap:  make(map[string]*model.ContainerInfo),
    }, nil
}

func (m *DockerManager) PullImage(ctx context.Context) error {
    log.Printf("Pulling Docker image: %s...", m.imageName)
    reader, err := m.cli.ImagePull(ctx, m.imageName, image.PullOptions{})
    if err != nil {
        log.Printf("Failed to pull Docker image %s: %v", m.imageName, err)
        return err
    }
    defer reader.Close()
    //TODO : Consider using alternative for verbose output
    if _, err := io.Copy(os.Stdout, reader); err != nil {
        log.Printf("Warning: Failed to copy image pull output: %v", err)
    }
    log.Printf("Image %s pulled successfully or already exists.", m.imageName)
    return nil
}

func (m *DockerManager) StartInitialContainers(ctx context.Context, count int) error {
    log.Printf("Creating and starting %d initial containers...", count)
    if count <= 0 {
        return fmt.Errorf("Invalid number of containers to start: %d", count)
    }

    m.availablePool = make(chan *model.ContainerInfo, count)
    m.containerMap = make(map[string]*model.ContainerInfo, count)

    var wg sync.WaitGroup
    var startMutex sync.Mutex
    errChan := make(chan error, count)

    for i:=0;i < count; i++ {
        wg.Add(1)
        go func(containerIndex int){
            defer wg.Done()
            log.Printf("Creating container %d...", containerIndex)
            resp, err := m.cli.ContainerCreate(ctx, &container.Config{
                Image: m.imageName,
                Cmd:   []string{"sleep", "infinity"},
                Tty: false,
            },nil,nil,nil,"")

            if err != nil {
                errChan <- fmt.Errorf("failed to create container %d: %w", containerIndex, err)
            }

            containerID := resp.ID
            err = m.cli.ContainerStart(ctx, containerID, container.StartOptions{})
            if err != nil {
                errChan <- fmt.Errorf("failed to start container %d: %w", containerIndex, err)
                rmCtx, rmCancel := context.WithTimeout(context.Background(), ContainerCleanupTimeout)
                defer rmCancel()
                rmErr := m.cli.ContainerRemove(rmCtx, containerID, container.RemoveOptions{Force: true})
                if rmErr != nil {
                    log.Printf("Warning: Failed to remove unstartable container %s: %v", containerID, rmErr)
                }
                return
            }

            log.Printf("Started container %d: %s", containerIndex, containerID)
            containInfo := &model.ContainerInfo{ID: containerID}
            startMutex.Lock()
            m.containers = append(m.containers, containInfo)
            m.containerMap[containerID] = containInfo
            startMutex.Unlock()

            m.availablePool <- containInfo
            log.Printf("Container %d added to available pool: %s", containerIndex, containerID)
        }(i)
    }

    wg.Wait()
    close(errChan)

    var startupErrors []error
    for err := range errChan {
        log.Printf("Error during container startup: %v", err)
        startupErrors = append(startupErrors, err)
    }

    m.containerMapMutex.Lock()
    numStarted := len(m.containers)
    m.containerMapMutex.Unlock()

    if numStarted == 0 {
        log.Fatal("Fatal: No containers were started successfully.")
    }

    if len(startupErrors) > 0 {
        log.Printf("Warning: Some containers failed to start: %v", startupErrors)
    }
    log.Printf("%d containers started successfully.", numStarted)
    return nil;
}

func (m *DockerManager) AcquireContainer(ctx context.Context) (*model.ContainerInfo, error) {
    select {
    case container := <-m.availablePool:
        log.Printf("Container %s acquired.", container.ID)
        return container, nil
    case <-ctx.Done():
        log.Printf("Context cancelled while waiting for container: %v", ctx.Err())
        return nil, fmt.Errorf("failed to acquire container: %w", ctx.Err())
    }
}
func (m *DockerManager) ReleaseContainer(container *model.ContainerInfo) {
    if container == nil {
        log.Println("Warning: Attempted to release a nil container.")
        return
    }
    log.Printf("Releasing container: %s", container.ID)
    select {
    case m.availablePool <- container:
        log.Printf("Container %s returned to pool.", container.ID)
    default:
        log.Printf("Warning: Could not return container %s to pool (pool might be full or closed).", container.ID)
    }
}

func (m *DockerManager) GetContainers() []*model.ContainerInfo {
    m.containerMapMutex.RLock()
    defer m.containerMapMutex.RUnlock()

    listCopy := make([]*model.ContainerInfo, len(m.containers))
    copy(listCopy, m.containers)
    return listCopy
}

func (m *DockerManager) CleanupContainers() {
    m.containerMapMutex.Lock()
    defer m.containerMapMutex.Unlock()


    if len(m.containers) == 0 {
        log.Println("No containers managed by this manager to clean up.")
        return
    }

    log.Println("Stopping and removing managed containers...")
    close(m.availablePool) // Close the channel to prevent new acquisitions 

    log.Printf("Stopping and removing %d containers...", len(m.containers))

    cleanupTimeout := ContainerCleanupTimeout + (time.Second * time.Duration(len(m.containers)))

    cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), cleanupTimeout)
    defer cancelCleanup()


    var wg sync.WaitGroup
    for _, c := range m.containers {
        wg.Add(1)
        go func(c *model.ContainerInfo){
            defer wg.Done()
            containerID := c.ID
            log.Printf("Stopping container %s...", containerID)

            stopTimeoutSecs := int(ContainerStopTimeout.Seconds())
            stopOpts := container.StopOptions{Timeout: &stopTimeoutSecs}

            if err := m.cli.ContainerStop(cleanupCtx, containerID, stopOpts); err != nil {
                if cleanupCtx.Err() != nil {
                    log.Printf("Context cancelled before stopping container %s: %v", containerID, cleanupCtx.Err())
                } else {
                    log.Printf("Error stopping container %s: %v. Attempting removal anyway.", containerID, err)
                }
            } else {
                log.Printf("Container %s stopped.", containerID)
            }
            
            if cleanupCtx.Err() != nil {
                log.Printf("Context cancelled before removing container %s.", containerID)
                return // Don't attempt removal if context is done
            }
            log.Printf("Removing container %s...", containerID)
            removeOpts := container.RemoveOptions{Force: true} // Force remove if stop failed or timed out
            if err := m.cli.ContainerRemove(cleanupCtx, containerID, removeOpts); err != nil {
                if cleanupCtx.Err() != nil {
                    log.Printf("Context cancelled during removal of container %s: %v", containerID, cleanupCtx.Err())
                } else {
                    log.Printf("Error removing container %s: %v", containerID, err)
                }
            } else {
                log.Printf("Container %s removed.", containerID)
            }
        }(c)
    }
    wg.Wait()


    if cleanupCtx.Err() == context.DeadlineExceeded {
        log.Println("Warning: Container cleanup timed out.")
    } else {
        log.Println("Finished container cleanup.")
    }

    m.containers = []*model.ContainerInfo{}
    m.containerMap = make(map[string]*model.ContainerInfo)
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