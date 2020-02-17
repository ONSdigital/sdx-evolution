package main

import (
	"encoding/json"
	"fmt"
	"log"

	redis "github.com/ONSdigital/sdx-evolution/internal/redis"
)

const (
	// SurveyConfigCacheKey is the key containing the survey config in redis
	SurveyConfigCacheKey = "sdx_survey_config"
)

// Survey represents the config for a specific survey
type Survey struct {
	Name             string   `json:"name"`
	LongName         string   `json:"long_name"`
	Active           bool     `json:"active"`
	ValidInstruments []string `json:"valid_instruments"`
	Downstream       string   `json:"downstream"`
}

// SurveyConfig represents the package of all survey configs
type SurveyConfig struct {
	Surveys map[string]Survey `json:"surveys"`
}

// TODO in the real world, these will be provisioned into the cache via
//		a separate mechanism.
func getInitialSurveyConfig() ([]byte, error) {
	surveys := SurveyConfig{
		Surveys: map[string]Survey{
			"144": Survey{
				Name:             "ukis",
				LongName:         "United Kingdom Innovation Survey",
				Active:           true,
				ValidInstruments: []string{"0001"},
				Downstream:       "cora",
			},
			"134": Survey{
				Name:             "mwss",
				LongName:         "Monthly wages and salary survey",
				Active:           false, // INTENTIONAL FOR TESTING ROUTING!
				ValidInstruments: []string{"0005"},
				Downstream:       "commonsoftware",
			},
			"023": Survey{
				Name:             "mbs",
				LongName:         "Monthly business survey - Retail Sales Index",
				Active:           true,
				ValidInstruments: []string{"0203", "0205", "0102", "0112", "0213", "0215"},
				Downstream:       "commonsoftware",
			},
		},
	}
	return json.Marshal(&surveys)
}

func populateInitialSurveyConfig() error {
	log.Printf(`event="Attempting to populate initial survey config"`)
	config, err := getInitialSurveyConfig()
	if err != nil {
		return err
	}
	return redis.Set(SurveyConfigCacheKey, string(config), redisConn)
}

func getSurveyConfig() (*SurveyConfig, error) {
	log.Printf(`event="Attempting to fetch survey config"`)

	configString, err := redis.GetString(SurveyConfigCacheKey, redisConn)
	if err != nil {
		return nil, fmt.Errorf("failed to get survey config: %v", err)
	}

	var config SurveyConfig
	if err = json.Unmarshal([]byte(configString), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal survey config: %v", err)
	}

	return &config, nil
}
