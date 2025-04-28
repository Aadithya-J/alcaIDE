package docker

import (
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
    containers []model.ContainerInfo
    imageName  string
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
        containers: []model.ContainerInfo{},
        imageName:  imageName,
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
    startedContainers := []model.ContainerInfo{}

    for i := 0; i < count; i++ {
        resp, err := m.cli.ContainerCreate(ctx, &container.Config{
            Image: m.imageName,
            Cmd:   []string{"sleep", "infinity"},
            Tty:   false,
        }, nil, nil, nil, "")

        if err != nil {
            log.Printf("Warning: Failed to create container %d: %v", i, err)
            continue
        }

        err = m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
        if err != nil {
            log.Printf("Warning: Failed to start container %s: %v", resp.ID, err)
            rmCtx, rmCancel := context.WithTimeout(context.Background(), ContainerCleanupTimeout)
            rmErr := m.cli.ContainerRemove(rmCtx, resp.ID, container.RemoveOptions{Force: true})
            if rmErr != nil {
                log.Printf("Warning: Failed to remove unstartable container %s: %v", resp.ID, rmErr)
            }
            rmCancel()
            continue
        }
        log.Printf("Started container %d: %s", i, resp.ID)

        startedContainers = append(startedContainers, model.ContainerInfo{ID: resp.ID, IsBusy: false})
    }

    m.containers = startedContainers

    if len(m.containers) == 0 {
        log.Fatal("Fatal: No containers were started successfully.")
    }

    log.Printf("%d containers started successfully.", len(m.containers))
    return nil
}

func (m *DockerManager) GetContainers() []model.ContainerInfo {
    listCopy := make([]model.ContainerInfo, len(m.containers))
    copy(listCopy, m.containers)
    return listCopy
}

func (m *DockerManager) CleanupContainers() {
    if len(m.containers) == 0 {
        log.Println("No containers managed by this manager to clean up.")
        return
    }

    log.Println("Stopping and removing managed containers...")
    cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), ContainerCleanupTimeout*time.Duration(len(m.containers))) // Adjust timeout based on container count
    defer cancelCleanup()

    var wg sync.WaitGroup
    for _, c := range m.containers {
        wg.Add(1)
        go func(containerID string) {
            defer wg.Done()
            log.Printf("Stopping container %s...", containerID)

            stopTimeoutSecs := int(ContainerStopTimeout.Seconds())
            stopOpts := container.StopOptions{Timeout: &stopTimeoutSecs}
            if err := m.cli.ContainerStop(cleanupCtx, containerID, stopOpts); err != nil {
                log.Printf("Error stopping container %s: %v. Attempting removal anyway.", containerID, err)
            } else {
                log.Printf("Container %s stopped.", containerID)
            }

            log.Printf("Removing container %s...", containerID)
            removeOpts := container.RemoveOptions{Force: true} // Force remove if stop failed
            if err := m.cli.ContainerRemove(cleanupCtx, containerID, removeOpts); err != nil {
                log.Printf("Error removing container %s: %v", containerID, err)
            } else {
                log.Printf("Container %s removed.", containerID)
            }
        }(c.ID)
    }
    wg.Wait()
    log.Println("Finished container cleanup.")
    m.containers = []model.ContainerInfo{}
}

func (m *DockerManager) Close() {
    log.Println("Closing Docker Client...")
    if err := m.cli.Close(); err != nil {
        log.Printf("Error closing Docker client: %v", err)
    } else {
        log.Println("Docker client closed successfully.")
    }
}