package handler

import (
	"context"
	"net/http"
	"encoding/json"
	"log"
	"time"
	"fmt"
	"errors"

	"github.com/Aadithya-J/alcaIDE/internal/docker"
	"github.com/Aadithya-J/alcaIDE/model"
)

const ACQUIRE_TIMEOUT = 10 * time.Second
const EXECUTION_TIMEOUT = 10 * time.Second

func ExecCodeHandler(w http.ResponseWriter, r* http.Request, parentCtx context.Context, dockerManager *docker.DockerManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var requestData struct {
		Code string `json:"code"`
		Language string `json:"language"`
	}

	cli := dockerManager.GetClient()

	err := json.NewDecoder(r.Body).Decode(&requestData);
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var execCmd []string
    switch requestData.Language {
    case "python":
        execCmd = []string{"python", "-c", requestData.Code}
    case "javascript":
        execCmd = []string{"node", "-e", requestData.Code}
    default:
        http.Error(w, fmt.Sprintf("Unsupported language: %s", requestData.Language), http.StatusBadRequest)
        return
    }


	var acquiredContainer *model.ContainerInfo
	acquireCtx, cancel := context.WithTimeout(parentCtx, ACQUIRE_TIMEOUT)
	defer cancel()

	log.Println("Acquiring container...")
	acquiredContainer, err = dockerManager.AcquireContainer(acquireCtx,requestData.Language)
	if err != nil {
		log.Println("Error acquiring container:", err)

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(acquireCtx.Err(), context.DeadlineExceeded) {
            http.Error(w, fmt.Sprintf("Container acquisition timed out for %s", requestData.Language), http.StatusRequestTimeout)
        } else {
            http.Error(w, fmt.Sprintf("Failed to acquire container for %s", requestData.Language), http.StatusInternalServerError)
        }
        return
	}
	log.Printf("Acquired container: %s\n", acquiredContainer.ID)

	defer dockerManager.ReleaseContainer(acquiredContainer, requestData.Language)

    execCtx, cancelExec := context.WithTimeout(parentCtx, EXECUTION_TIMEOUT)
    defer cancelExec()

    log.Printf("Executing %s code in container %s...", requestData.Language, acquiredContainer.ID)

    output, err := acquiredContainer.ExecuteCode(execCmd, cli, execCtx)
    var errMsg string

    if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
        errMsg = fmt.Sprintf("Execution timed out after %s", EXECUTION_TIMEOUT)
        log.Printf("Execution timed out for container %s (%s)", acquiredContainer.ID, requestData.Language)
        w.WriteHeader(http.StatusRequestTimeout)
        respondJSON(w, model.ExecResponse{
            Code:     requestData.Code,
            Language: requestData.Language,
            Output:   "",
            Error:    errMsg,
        })
        return
    }
    if err != nil {
        errMsg = err.Error()
        log.Printf("Execution error in container %s (%s): %s", acquiredContainer.ID, requestData.Language, errMsg)
		w.WriteHeader(http.StatusBadRequest)
    } else {
        log.Printf("Execution successful in container %s (%s)", acquiredContainer.ID, requestData.Language)
    }

    respondJSON(w, model.ExecResponse{
        Code:     requestData.Code,
        Language: requestData.Language,
        Output:   output,
        Error:    errMsg,
    })
}