package notificationevents

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestJobRedaction(t *testing.T) {
	job := Job{LeaseToken: "synthetic-private-lease"}
	encoded, _ := json.Marshal(job)
	for _, output := range []string{
		fmt.Sprintf("%+v", job), string(encoded), job.LogValue().String(),
	} {
		if strings.Contains(output, job.LeaseToken) {
			t.Fatal("outbox lease leaked")
		}
	}
}
