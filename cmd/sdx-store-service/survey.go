package main

// Survey represents the key elements of a block of survey JSON that can be
// processed by this service. It only attempts to map the common core elements
// that should always be present no matter the survey type.
type Survey struct {
	TxID     string `json:"tx_id"`
	Type     string `json:"type"`
	Origin   string `json:"origin"`
	SurveyID string `json:"survey_id"`
}
