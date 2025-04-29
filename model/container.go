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

	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)
	if err != nil && err != io.EOF {
		log.Printf("warning: output copy might be incomplete: %v", err)
	}

	inspectResp, err := cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		log.Printf("warning: exec inspect failed: %v", err)
	}

	output := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if inspectResp.ExitCode != 0 {
		combinedOutput := output
		if stderrStr != "" {
			if combinedOutput != "" {
				combinedOutput += "\n"
			}
			combinedOutput += "Stderr:\n" + stderrStr
		}
		return output, fmt.Errorf(
			"python execution failed (exit code %d):\n%s",
			inspectResp.ExitCode,
			combinedOutput,
		)
	} else if stderrStr != "" {
		log.Printf("Python execution produced stderr (exit code 0) in container %s:\n%s", c.ID, stderrStr)
	}

	return output, nil
}