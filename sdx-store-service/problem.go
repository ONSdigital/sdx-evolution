package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// HTTPProblemResponse as specified in RFC7807 - https://tools.ietf.org/html/rfc7807
type HTTPProblemResponse struct {
	Type   string `json:"type,omitempty"`   // Link to a resource for the problem
	Title  string `json:"title,omitempty"`  // Short description of the issue
	Status int    `json:"status,omitempty"` // The http status code
	Detail string `json:"detail,omitempty"` // Further human-readable detail
}

func writeProblemResponse(problem HTTPProblemResponse, rw http.ResponseWriter) {
	pr, err := json.Marshal(&problem)
	if err != nil {
		log.Printf(`event="Error writing problem reponse" error="%v"`, err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/problem+json")
	rw.Header().Set("Content-Language", "en")
	rw.WriteHeader(problem.Status)
	rw.Write(pr)
	return
}
