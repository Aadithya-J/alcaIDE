package handler

import (
	"context"
	"net/http"
	"encoding/json"
	"log"
	"time"
	"fmt"

	"github.com/Aadithya-J/alcaIDE/internal/docker"
	"github.com/Aadithya-J/alcaIDE/model"
)

const ACQUIRE_TIMEOUT = 10 * time.Second
const EXECUTION_TIMEOUT = 10 * time.Second

func ExecPythonHandler(w http.ResponseWriter, r* http.Request, parentCtx context.Context, dockerManager *docker.DockerManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var requestData struct {
		Code string `json:"code"`
	}

	cli := dockerManager.GetClient()

	err := json.NewDecoder(r.Body).Decode(&requestData);
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var acquiredContainer *model.ContainerInfo
	acquireCtx, cancel := context.WithTimeout(parentCtx, ACQUIRE_TIMEOUT)
	defer cancel()

	log.Println("Acquiring container...")
	acquiredContainer, err = dockerManager.AcquireContainer(acquireCtx)
	if err != nil {
		log.Println("Error acquiring container:", err)

		if err == context.DeadlineExceeded {
			http.Error(w, "Container acquisition timed out", http.StatusRequestTimeout)
		} else {
			http.Error(w, "Failed to acquire container", http.StatusInternalServerError)
		}
		return
	}
	log.Printf("Acquired container: %s\n", acquiredContainer.ID)

	defer dockerManager.ReleaseContainer(acquiredContainer)

	execCtx, cancel := context.WithTimeout(parentCtx, EXECUTION_TIMEOUT) // Use the constant
	defer cancel()
	log.Println("Executing Python code...")
	// Use execCtx for the execution call
	output, err := acquiredContainer.ExecutePython(requestData.Code, cli, execCtx)
	var errMsg string

	if execCtx.Err() == context.DeadlineExceeded {
		errMsg := fmt.Sprintf("Execution timed out after %s", EXECUTION_TIMEOUT)
		log.Printf("Execution timed out for container %s", acquiredContainer.ID)
		w.WriteHeader(http.StatusRequestTimeout)
		respondJSON(w, model.ExecResponse{
			Code:   requestData.Code,
			Output: "",
			Error:  errMsg,
		})
		return
	}

	if err != nil {
		errMsg = err.Error()
		log.Printf("Execution error in container %s: %s", acquiredContainer.ID, errMsg)
	} else {
		log.Printf("Execution successful in container %s", acquiredContainer.ID)
	}

	respondJSON(w, model.ExecResponse{
		Code:   requestData.Code,
		Output: output,
		Error:  errMsg, // Include error message in JSON if one occurred (and wasn't a timeout)
	})
}