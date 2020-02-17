package main

import (
	"testing"
)

func TestGetHealth(t *testing.T) {

	// Empty health
	currentHealth = nil
	_, err := getHealth()
	if err == nil {
		t.Error("Expected error when getting health with no health set")
	}

	// Health set
	currentHealth = &health{Service: true}
	h, err := getHealth()
	if err != nil {
		t.Error("Expected no error when getting health")
	}
	if h == nil {
		t.Error("Expected content returned when getting health")
	}

}
