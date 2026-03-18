package handler

import "net/http"

func Health(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "api-server"})
}
