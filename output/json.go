package output

import (
	"encoding/json"
	"fmt"
	"github.com/pkositsyn/traceroute-go/trace"
	"io"
)

type JsonFormatter struct {}

var _ Formatter = (*JsonFormatter)(nil)

func (j *JsonFormatter) Format(channel *trace.ResponseChannel) error {
	resultRows := make([]*trace.ResponseForTTL, 0)
	for {
		row, err := channel.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		resultRows = append(resultRows, row)
	}
	result := trace.AssembleResponse{Data: resultRows}

	jsonOutput, err := json.Marshal(result)
	if err != nil {
		return err
	}

	fmt.Println(string(jsonOutput))
	return nil
}
