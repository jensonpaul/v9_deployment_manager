package worker

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"v9_deployment_manager/log"
)

type V9Worker struct {
	URL string
}

type ComponentPath struct {
	User string `json:"user"`
	Repo string `json:"repo"`
}

type ComponentID struct {
	User string `json:"user"`
	Repo string `json:"repo"`
	Hash string `json:"hash"`
}

type activateRequest struct {
	ID              ComponentID `json:"id"`
	ExecutableFile  string      `json:"executable_file"`
	ExecutionMethod string      `json:"execution_method"`
}

func createActivateBody(compID ComponentID, tarPath string, executionMethod string) ([]byte, error) {
	body, err := json.Marshal(activateRequest{compID, tarPath, executionMethod})
	return body, err
}

type deactivateRequest struct {
	ID ComponentID `json:"id"`
}

// Build activate post body
func createDeactivateBody(compID ComponentID) ([]byte, error) {
	body, err := json.Marshal(deactivateRequest{compID})
	return body, err
}

type ComponentStats struct {
	ID ComponentID `json:"id"`

	Color      string  `json:"color"`
	StatWindow float64 `json:"stat_window_seconds"`

	Hits float64 `json:"hits"`

	AvgResponseBytes   float64   `json:"avg_response_bytes"`
	AvgMsLatency       float64   `json:"avg_ms_latency"`
	LatencyPercentiles []float64 `json:"ms_latency_percentiles"`
}

type StatusResponse struct {
	CPUUsage         float64          `json:"cpu_usage"`
	MemoryUsage      float64          `json:"memory_usage"`
	NetworkUsage     float64          `json:"network_usage"`
	ActiveComponents []ComponentStats `json:"active_components"`
}

func contains(l []ComponentPath, v ComponentID) bool {
	for _, comp := range l {
		if comp.Repo == v.Repo && comp.User == v.User {
			return true
		}
	}
	return false
}

func (s *StatusResponse) FindNonactive(activeComponents []ComponentPath) []ComponentID {
	var nonActive = make([]ComponentID, 0)

	for _, runningComponent := range s.ActiveComponents {
		if !contains(activeComponents, runningComponent.ID) {
			nonActive = append(nonActive, runningComponent.ID)
		}
	}

	return nonActive
}

func (s *StatusResponse) ContainsPath(compPath ComponentPath) bool {
	for _, runningComponent := range s.ActiveComponents {
		if runningComponent.ID.User == compPath.User && runningComponent.ID.Repo == compPath.Repo {
			return true
		}
	}
	return false
}

func (s *StatusResponse) ContainsExactly(compID ComponentID) bool {
	for _, runningComponent := range s.ActiveComponents {
		if runningComponent.ID == compID {
			return true
		}
	}
	return false
}

type ComponentLog struct {
	ID          ComponentID `json:"id"`
	DedupNumber uint64      `json:"dedup_number"`
	Log         *string     `json:"log"`
	Error       *string     `json:"error"`
}

type LogResponse struct {
	Logs []ComponentLog `json:"logs"`
}

func (worker *V9Worker) post(route string, body []byte) (*http.Response, error) {
	url := "http://" + worker.URL + route
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Error.Println("Failed to post", err)
		return nil, err
	}

	return resp, nil
}

func (worker *V9Worker) get(route string) (*http.Response, error) {
	url := "http://" + worker.URL + route
	resp, err := http.Get(url)
	if err != nil {
		log.Error.Println("Failed to get", err)
		return nil, err
	}

	return resp, nil
}

func (worker *V9Worker) Activate(component ComponentID, tarPath string) error {
	// Marshal information into json body
	body, err := createActivateBody(component, tarPath, "docker-archive")
	if err != nil {
		log.Error.Println("Failed to create activation body", err)
		return err
	}

	// Make activate post request
	resp, err := worker.post("/meta/activate", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error.Println("Failure to read response from worker", err)
		return err
	}

	// TODO: Look for activate errors and store them somewhere
	log.Info.Println("Response from worker:", string(respBody))
	return nil
}

func (worker *V9Worker) Deactivate(component ComponentID) error {
	// Marshal information into json body
	body, err := createDeactivateBody(component)
	if err != nil {
		log.Error.Println("Failed to create deactivation body", err)
		return err
	}

	// Make deactivate post request
	resp, err := worker.post("/meta/deactivate", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error.Println("Failure to read response from worker", err)
		return err
	}

	// TODO: Look for deactivate errors and store them somewhere
	log.Info.Println("Response from worker:", string(respBody))
	return nil
}

// Deactivate component
func DeactivateComponentEverywhere(compID ComponentID, workers []*V9Worker) {
	for i := range workers {
		err := workers[i].Deactivate(compID)
		if err != nil {
			log.Info.Println("Failed to deactivate worker:", i, err)
			// This can fail and should fall through
		}
	}
}

func (worker *V9Worker) Logs() (LogResponse, error) {
	url := "http://" + worker.URL + "/meta/logs"
	resp, err := http.Get(url)
	if err != nil {
		log.Error.Println("Failed to get logs", err)
		return LogResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error.Println("Failure to read response from worker", err)
		return LogResponse{}, err
	}

	var logResponse LogResponse
	err = json.Unmarshal(respBody, &logResponse)
	if err != nil {
		return LogResponse{}, err
	}

	return logResponse, nil
}

func (worker *V9Worker) Status() (StatusResponse, error) {
	resp, err := worker.get("/meta/status")
	if err != nil {
		log.Error.Println("Failed to get status", err)
		return StatusResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error.Println("Failure to read status response from worker", err)
		return StatusResponse{}, err
	}

	var statusResponse StatusResponse
	err = json.Unmarshal(respBody, &statusResponse)
	if err != nil {
		return StatusResponse{}, err
	}

	return statusResponse, nil
}
