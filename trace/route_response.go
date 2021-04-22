package trace

type AssembleResponse struct {
	Data []*ResponseForTTL `json:"data"`
}

func newAssembleResponse(maxTTL int) *AssembleResponse {
	response := new(AssembleResponse)
	response.Data = make([]*ResponseForTTL, maxTTL)
	for i, _ := range response.Data {
		response.Data[i] = &ResponseForTTL{
			TTL:          i + 1,
			PacketsForIP: make(map[string][]string),
		}
	}
	return response
}

type ResponseForTTL struct {
	TTL int `json:"ttl"`
	PacketsForIP map[string][]string `json:"responses_per_ip"`
}

type ResponseChannel struct {
	ch chan *ResponseForTTL
	errors chan error
}

func newResponseChannel() *ResponseChannel {
	return &ResponseChannel{
		ch:     make(chan *ResponseForTTL),
		errors: make(chan error),
	}
}

func (r *ResponseChannel) Next() (*ResponseForTTL, error) {
	select {
	case response := <-r.ch:
		return response, nil
	case err := <- r.errors:
		return nil, err
	}
}