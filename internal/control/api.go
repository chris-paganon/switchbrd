package control

import "dev-switchboard/internal/app"

type addRequest struct {
	Port int    `json:"port"`
	Name string `json:"name,omitempty"`
}

type activateRequest struct {
	Target string `json:"target"`
	Name   string `json:"name,omitempty"`
}

type renameRequest struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

type appResponse struct {
	App app.App `json:"app"`
}

type activeResponse struct {
	App *app.App `json:"app,omitempty"`
}

type listResponse struct {
	Apps       []app.App `json:"apps"`
	ActiveName string    `json:"active_name"`
}

type errorResponse struct {
	Error string `json:"error"`
}
