package main

import (
	"fmt"
	"reflect"

	"github.com/google/knative-gcp/hack/schema/pkg/schema"

	"gopkg.in/yaml.v3"

	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
)

func main() {
	t := reflect.TypeOf(v1.Topic{})
	//t = reflect.TypeOf(v1alpha1.BrokerCell{})
	s := schema.GenerateForType(t)
	b, _ := yaml.Marshal(s)
	fmt.Print(string(b))
}
