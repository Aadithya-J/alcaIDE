package model

type ExecResponse struct {
    Code     string `json:"code"`
    Language string `json:"language"`
    Output   string `json:"output"`
    Error    string `json:"error"`
}