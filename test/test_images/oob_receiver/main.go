/*
Copyright 2019 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/kelseyhightower/envconfig"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/kncloudevents"
	"github.com/google/knative-gcp/test/e2e/lib"
)

const (
	firstNErrsEnvVar = "FIRST_N_ERRS"
)

type envConfig struct {
	BrokerURL string `envconfig:"BROKER_URL" required:"true"`
}

type Receiver struct {
	client    cloudevents.Client
	errsCount int
	mux       sync.Mutex
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		panic(fmt.Sprintf("Failed to process env var: %s", err))
	}
	client, err := kncloudevents.NewDefaultClient(env.BrokerURL)
	if err != nil {
		panic(err)
	}

	r := &Receiver{
		client:    client,
		errsCount: 0,
	}
	if err := r.client.StartReceiver(context.Background(), r.Receive); err != nil {
		log.Fatal(err)
	}
}

func (r *Receiver) Receive(ctx context.Context, event cloudevents.Event) (*event.Event, protocol.Result) {

	// Check if the received event is the dummy event sent by sender pod.
	// If it is, send back a response CloudEvent.
	if event.ID() == lib.E2EDummyEventID {
		event = cloudevents.NewEvent(cloudevents.VersionV1)
		event.SetID(lib.E2EDummyRespEventID)
		event.SetType(lib.E2EDummyRespEventType)
		event.SetSource(lib.E2EDummyRespEventSource)
		event.SetData(cloudevents.ApplicationJSON, `{"source": "receiver!"}`)
		err := r.sendEvent(event)
		if err != nil {
			// Ksvc seems to auto retry 5xx. So use 4xx for predictability.
			return nil, cehttp.NewResult(http.StatusFailedDependency, "Unable to send cloud event")
		}
		return nil, cehttp.NewResult(http.StatusAccepted, "OK")
	} else {
		return nil, cehttp.NewResult(http.StatusForbidden, "Forbidden")
	}
}

func (r *Receiver) sendEvent(event cloudevents.Event) error {
	ctx := cloudevents.WithEncodingBinary(context.Background())
	result := r.client.Send(ctx, event)
	if cloudevents.IsACK(result) {
		return nil
	}
	return fmt.Errorf("event send did not ACK: %w", result)
}
