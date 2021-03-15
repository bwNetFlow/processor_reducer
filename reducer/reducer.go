package reducer

import (
	"log"
	"net"
	"reflect"
	"strings"

	bwmessage "github.com/bwNetFlow/protobuf/go"
)

// Reducer stores the reduction specific configuration.
type Reducer struct {
	// Fields which will be kept
	LimitFields string
	// Fields which will be anonymized, if kept by fields.limit
	AnonFields string
}

// process does the actual reduction work.
func (r *Reducer) Process(msg *bwmessage.FlowMessage) *bwmessage.FlowMessage {
	reflected_original := reflect.ValueOf(msg) // immutable
	reduced := bwmessage.FlowMessage{}
	reflected_reduced := reflect.ValueOf(&reduced) // mutable
	// limit stuff
	for _, fieldname := range strings.Split(r.LimitFields, ",") {
		if fieldname == "" {
			log.Fatal("reducer.fields.limit unset, terminating...")
		}
		original_field := reflect.Indirect(reflected_original).FieldByName(fieldname)
		reduced_field := reflected_reduced.Elem().FieldByName(fieldname)
		if original_field.IsValid() && reduced_field.IsValid() {
			reduced_field.Set(original_field)
		} else {
			log.Printf("Flow messages do not have a field named '%s'", fieldname)
		}
	}
	// anon stuff
	for _, fieldname := range strings.Split(r.AnonFields, ",") {
		if fieldname == "" {
			continue // case anonFields == ''
		}
		reduced_field := reflected_reduced.Elem().FieldByName(fieldname)
		if reduced_field.IsValid() {
			if reduced_field.Type() == reflect.TypeOf([]uint8{}) {
				raw := reduced_field.Interface().([]uint8)
				address := net.IP(raw)
				raw[len(raw)-1] = 0
				if address.To4() == nil {
					for i := 2; i <= 8; i++ {
						raw[len(raw)-i] = 0
					}
				}
				reduced_field.Set(reflect.ValueOf(raw))
			} else {
				log.Printf("Field '%s' has type '%s'. Anonymization is only supported for IP types.", fieldname, reduced_field.Type())
			}
		} else {
			log.Printf("The reduced flow message did not have a field named '%s' to anonymize.", fieldname)
		}
	}
	return &reduced
}
