/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	ecc "github.com/ernestio/ernest-config-client"
	"github.com/nats-io/nats"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	testEvent = Event{
		UUID:                  "test",
		BatchID:               "test",
		ProviderType:          "aws",
		VPCID:                 "vpc-0000000",
		DatacenterRegion:      "eu-west-1",
		DatacenterAccessKey:   "key",
		DatacenterAccessToken: "token",
		NetworkAWSID:          "subnet-00000000",
		PublicNetworkAWSID:    "subnet-00000000",
		RoutedNetworkAWSIDs:   []string{"subnet-00000001"},
		NatGatewayAWSID:       "nat-00000000",
	}
)

func waitMsg(ch chan *nats.Msg) (*nats.Msg, error) {
	select {
	case msg := <-ch:
		return msg, nil
	case <-time.After(time.Millisecond * 100):
	}
	return nil, errors.New("timeout")
}

func testSetup(subject string) (chan *nats.Msg, chan *nats.Msg) {
	doneChan := make(chan *nats.Msg, 10)
	errChan := make(chan *nats.Msg, 10)

	nc = ecc.NewConfig(os.Getenv("NATS_URI")).Nats()

	nc.ChanSubscribe(subject+".done", doneChan)
	nc.ChanSubscribe(subject+".error", errChan)

	return doneChan, errChan
}

func TestEventCreation(t *testing.T) {
	subject := "nat.create.aws"
	completed, errored := testSetup(subject)

	Convey("Given I an event", t, func() {
		Convey("With valid fields", func() {
			valid, _ := json.Marshal(testEvent)
			Convey("When processing the event", func() {
				var e Event
				err := e.Process(subject, valid)

				Convey("It should not error", func() {
					So(err, ShouldBeNil)
					msg, timeout := waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})

				Convey("It should load the correct values", func() {
					So(e.UUID, ShouldEqual, "test")
					So(e.BatchID, ShouldEqual, "test")
					So(e.ProviderType, ShouldEqual, "aws")
					So(e.VPCID, ShouldEqual, "vpc-0000000")
					So(e.DatacenterRegion, ShouldEqual, "eu-west-1")
					So(e.DatacenterAccessKey, ShouldEqual, "key")
					So(e.DatacenterAccessToken, ShouldEqual, "token")
					So(e.PublicNetworkAWSID, ShouldEqual, "subnet-00000000")
					So(len(e.RoutedNetworkAWSIDs), ShouldEqual, 1)
					So(e.RoutedNetworkAWSIDs[0], ShouldEqual, "subnet-00000001")
				})
			})

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, valid)
				err := e.Validate(subject)

				Convey("It should not error", func() {
					So(err, ShouldBeNil)
					msg, timeout := waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
			})

			Convey("When completing the event", func() {
				var e Event
				e.Process(subject, valid)
				e.Complete()
				Convey("It should produce a nat.create.aws.done event", func() {
					msg, timeout := waitMsg(completed)
					So(msg, ShouldNotBeNil)
					So(string(msg.Data), ShouldEqual, string(valid))
					So(timeout, ShouldBeNil)
					msg, timeout = waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
			})

			Convey("When erroring the event", func() {
				log.SetOutput(ioutil.Discard)
				var e Event
				e.Process(subject, valid)
				e.Error(errors.New("error"))
				Convey("It should produce a nat.create.aws.error event", func() {
					msg, timeout := waitMsg(errored)
					So(msg, ShouldNotBeNil)
					So(string(msg.Data), ShouldContainSubstring, `"error_message":"error"`)
					So(timeout, ShouldBeNil)
					msg, timeout = waitMsg(completed)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
				log.SetOutput(os.Stdout)
			})
		})

		Convey("With no datacenter vpc id", func() {
			testEventInvalid := testEvent
			testEventInvalid.VPCID = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter VPC ID invalid")
				})
			})
		})

		Convey("With no datacenter region", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterRegion = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter Region invalid")
				})
			})
		})

		Convey("With no datacenter access key", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterAccessKey = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter credentials invalid")
				})
			})
		})

		Convey("With no datacenter access token", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterAccessToken = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter credentials invalid")
				})
			})
		})

		Convey("With no network id", func() {
			testEventInvalid := testEvent
			testEventInvalid.PublicNetworkAWSID = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Network id invalid")
				})
			})
		})

		Convey("With no routed network id's", func() {
			testEventInvalid := testEvent
			testEventInvalid.RoutedNetworkAWSIDs = []string{}
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Routed networks are empty")
				})
			})
		})

	})
}

func TestEventDeletion(t *testing.T) {
	subject := "nat.delete.aws"
	completed, errored := testSetup(subject)

	Convey("Given I an event", t, func() {
		Convey("With valid fields", func() {
			valid, _ := json.Marshal(testEvent)
			Convey("When processing the event", func() {
				var e Event
				err := e.Process(subject, valid)

				Convey("It should not error", func() {
					So(err, ShouldBeNil)
					msg, timeout := waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})

				Convey("It should load the correct values", func() {
					So(e.UUID, ShouldEqual, "test")
					So(e.BatchID, ShouldEqual, "test")
					So(e.ProviderType, ShouldEqual, "aws")
					So(e.VPCID, ShouldEqual, "vpc-0000000")
					So(e.DatacenterRegion, ShouldEqual, "eu-west-1")
					So(e.DatacenterAccessKey, ShouldEqual, "key")
					So(e.DatacenterAccessToken, ShouldEqual, "token")
					So(e.NetworkAWSID, ShouldEqual, "subnet-00000000")
				})
			})

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, valid)
				err := e.Validate(subject)

				Convey("It should not error", func() {
					So(err, ShouldBeNil)
					msg, timeout := waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
			})

			Convey("When completing the event", func() {
				var e Event
				e.Process(subject, valid)
				e.Complete()
				Convey("It should produce a nat.delete.aws.done event", func() {
					msg, timeout := waitMsg(completed)
					So(msg, ShouldNotBeNil)
					So(string(msg.Data), ShouldEqual, string(valid))
					So(timeout, ShouldBeNil)
					msg, timeout = waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
			})

			Convey("When erroring the event", func() {
				log.SetOutput(ioutil.Discard)
				var e Event
				e.Process(subject, valid)
				e.Error(errors.New("error"))
				Convey("It should produce a nat.delete.aws.error event", func() {
					msg, timeout := waitMsg(errored)
					So(msg, ShouldNotBeNil)
					So(string(msg.Data), ShouldContainSubstring, `"error_message":"error"`)
					So(timeout, ShouldBeNil)
					msg, timeout = waitMsg(completed)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
				log.SetOutput(os.Stdout)
			})
		})

		Convey("With no datacenter vpc id", func() {
			testEventInvalid := testEvent
			testEventInvalid.VPCID = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter VPC ID invalid")
				})
			})
		})

		Convey("With no datacenter region", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterRegion = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter Region invalid")
				})
			})
		})

		Convey("With no datacenter access key", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterAccessKey = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter credentials invalid")
				})
			})
		})

		Convey("With no datacenter access token", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterAccessToken = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter credentials invalid")
				})
			})
		})

		Convey("With no nat aws id", func() {
			testEventInvalid := testEvent
			testEventInvalid.NatGatewayAWSID = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Nat Gateway aws id invalid")
				})
			})
		})

	})
}

func TestEventModification(t *testing.T) {
	subject := "nat.update.aws"
	completed, errored := testSetup(subject)

	Convey("Given I an event", t, func() {
		Convey("With valid fields", func() {
			valid, _ := json.Marshal(testEvent)
			Convey("When processing the event", func() {
				var e Event
				err := e.Process(subject, valid)

				Convey("It should not error", func() {
					So(err, ShouldBeNil)
					msg, timeout := waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})

				Convey("It should load the correct values", func() {
					So(e.UUID, ShouldEqual, "test")
					So(e.BatchID, ShouldEqual, "test")
					So(e.ProviderType, ShouldEqual, "aws")
					So(e.VPCID, ShouldEqual, "vpc-0000000")
					So(e.DatacenterRegion, ShouldEqual, "eu-west-1")
					So(e.DatacenterAccessKey, ShouldEqual, "key")
					So(e.DatacenterAccessToken, ShouldEqual, "token")
					So(e.PublicNetworkAWSID, ShouldEqual, "subnet-00000000")
					So(len(e.RoutedNetworkAWSIDs), ShouldEqual, 1)
					So(e.RoutedNetworkAWSIDs[0], ShouldEqual, "subnet-00000001")
				})
			})

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, valid)
				err := e.Validate(subject)

				Convey("It should not error", func() {
					So(err, ShouldBeNil)
					msg, timeout := waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
			})

			Convey("When completing the event", func() {
				var e Event
				e.Process(subject, valid)
				e.Complete()
				Convey("It should produce a nat.update.aws.done event", func() {
					msg, timeout := waitMsg(completed)
					So(msg, ShouldNotBeNil)
					So(string(msg.Data), ShouldEqual, string(valid))
					So(timeout, ShouldBeNil)
					msg, timeout = waitMsg(errored)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
			})

			Convey("When erroring the event", func() {
				log.SetOutput(ioutil.Discard)
				var e Event
				e.Process(subject, valid)
				e.Error(errors.New("error"))
				Convey("It should produce a nat.update.aws.error event", func() {
					msg, timeout := waitMsg(errored)
					So(msg, ShouldNotBeNil)
					So(string(msg.Data), ShouldContainSubstring, `"error_message":"error"`)
					So(timeout, ShouldBeNil)
					msg, timeout = waitMsg(completed)
					So(msg, ShouldBeNil)
					So(timeout, ShouldNotBeNil)
				})
				log.SetOutput(os.Stdout)
			})
		})

		Convey("With no datacenter vpc id", func() {
			testEventInvalid := testEvent
			testEventInvalid.VPCID = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter VPC ID invalid")
				})
			})
		})

		Convey("With no datacenter region", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterRegion = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter Region invalid")
				})
			})
		})

		Convey("With no datacenter access key", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterAccessKey = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter credentials invalid")
				})
			})
		})

		Convey("With no datacenter access token", func() {
			testEventInvalid := testEvent
			testEventInvalid.DatacenterAccessToken = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Datacenter credentials invalid")
				})
			})
		})

		Convey("With no network id", func() {
			testEventInvalid := testEvent
			testEventInvalid.PublicNetworkAWSID = ""
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Network id invalid")
				})
			})
		})

		Convey("With no routed network id's", func() {
			testEventInvalid := testEvent
			testEventInvalid.RoutedNetworkAWSIDs = []string{}
			invalid, _ := json.Marshal(testEventInvalid)

			Convey("When validating the event", func() {
				var e Event
				e.Process(subject, invalid)
				err := e.Validate(subject)
				Convey("It should error", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldEqual, "Routed networks are empty")
				})
			})
		})

	})
}
