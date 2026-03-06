package control

import "dev-switchboard/internal/app"

type addRequest struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

type activateRequest struct {
	Name string `json:"name"`
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
