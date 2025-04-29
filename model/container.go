package model

import (
	"sync"
	"bytes"
	"context"
	"log"
	"io"
	"fmt"
    
	
	"github.com/docker/docker/api/types/container"
	// "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)


type ContainerInfo struct {
	ID     string
	IsBusy bool
	mu     sync.Mutex
}

// Method to safely check and acquire the container if free
func (c *ContainerInfo) TryAcquire() bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.IsBusy {
        return false
    }
    c.IsBusy = true
    return true 
}

// Method to safely release the container
func (c *ContainerInfo) Release() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.IsBusy = false 
}
func (c *ContainerInfo) ExecutePython(code string, cli *client.Client, ctx context.Context) (string, error) {
    execConfig := container.ExecOptions{
        Cmd:          []string{"python", "-c", code},
        AttachStdout: true,
        AttachStderr: true,
    }
	fmt.Println("Executing Python code in container:", c.ID)
	fmt.Println(code)

    execResp, err := cli.ContainerExecCreate(ctx, c.ID, execConfig)
    if err != nil {
        return "", fmt.Errorf("exec create failed: %w", err)
    }

    attachResp, err := cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
    if err != nil {
        return "", fmt.Errorf("exec attach failed: %w", err)
    }
    defer attachResp.Close()

    var stdoutBuf, stderrBuf bytes.Buffer

    _, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)
    if err != nil && err != io.EOF {
        log.Printf("warning: output copy might be incomplete: %v", err)
    }

    inspectResp, err := cli.ContainerExecInspect(ctx, execResp.ID)
    if err != nil {
        return "", fmt.Errorf("exec inspect failed: %w", err)
    }

    output := stdoutBuf.String()
    if inspectResp.ExitCode != 0 {
        return output, fmt.Errorf(
            "python execution failed (exit code %d):\nStdout:\n%s\nStderr:\n%s",
            inspectResp.ExitCode,
            output,
            stderrBuf.String(),
        )
    }

    return output, nil
}