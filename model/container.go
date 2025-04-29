package model

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type ContainerInfo struct {
	ID string
}

func (c *ContainerInfo) ExecutePython(code string, cli *client.Client, ctx context.Context) (string, error) {
	execConfig := container.ExecOptions{
		Cmd:          []string{"python", "-c", code},
		AttachStdout: true,
		AttachStderr: true,
	}
	fmt.Println("Executing Python code in container:", c.ID)

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
	copyErr := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)
		if err != nil && err != io.EOF {
			copyErr <- err
		} else {
			copyErr <- nil
		}
	}()

	select {
	case <-ctx.Done():
		attachResp.Close()
		return "", fmt.Errorf("python execution timed out: %w", ctx.Err())
	case err := <-copyErr:
		if err != nil {
			log.Printf("warning: stdcopy incomplete: %v", err)
		}
	}

	inspectResp, err := cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		log.Printf("warning: exec inspect failed: %v", err)
	}

	outStr := stdoutBuf.String()
	errStr := stderrBuf.String()
	if inspectResp.ExitCode != 0 {
		combined := outStr
		if errStr != "" {
			if combined != "" {
				combined += "\n"
			}
			combined += "Stderr:\n" + errStr
		}
		return outStr, fmt.Errorf("python execution failed (exit %d):\n%s", inspectResp.ExitCode, combined)
	}
	if errStr != "" {
		log.Printf("python stderr (exit 0) in %s:\n%s", c.ID, errStr)
	}
	return outStr, nil
}