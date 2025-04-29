package model

type ExecResponse struct {
	Code string `json:"code"`
	Output string `json:"output"`
	Error string `json:"error"`
}