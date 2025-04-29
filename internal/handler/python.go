package handler

import (
	"context"

	"net/http"
	"encoding/json"

	"github.com/Aadithya-J/alcaIDE/model"
	"github.com/docker/docker/client"
)


func ExecPythonHandler(w http.ResponseWriter, r* http.Request, cli *client.Client, ctx context.Context, containers []model.ContainerInfo) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestData struct {
		Code string `json:"code"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestData);
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var acquiredContainer *model.ContainerInfo
	for i := range containers {
		container := &containers[i]
		if container.TryAcquire() {
			acquiredContainer = container
			break
		}
	}

	if acquiredContainer == nil {
		http.Error(w, "No available containers", http.StatusServiceUnavailable)
		return
	}

	defer acquiredContainer.Release()

	output,err := acquiredContainer.ExecutePython(requestData.Code, cli, ctx)
	var errMsg string
    if err != nil {
        errMsg = err.Error()
    }
	respondJSON(w, model.ExecResponse{
		Code: requestData.Code,
		Output: output,
		Error: errMsg,
	})

}