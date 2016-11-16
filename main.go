/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	ecc "github.com/ernestio/ernest-config-client"
	"github.com/nats-io/nats"
)

var nc *nats.Conn
var natsErr error

func eventHandler(m *nats.Msg) {
	n := New(m.Subject, m.Data)

	err := n.Process()
	if err != nil {
		return
	}

	if err = n.Validate(); err != nil {
		n.Error(err)
		return
	}

	parts := strings.Split(m.Subject, ".")
	switch parts[1] {
	case "create":
		err = n.Create()
	case "update":
		err = n.Update()
	case "delete":
		err = n.Delete()
	case "get":
		err = n.Get()
	}
	if err != nil {
		n.Error(err)
		return
	}

	n.Complete()
}

func main() {
	nc = ecc.NewConfig(os.Getenv("NATS_URI")).Nats()

	events := []string{"nat.create.aws", "nat.update.aws", "nat.delete.aws"}
	for _, subject := range events {
		fmt.Println("listening for " + subject)
		nc.Subscribe(subject, eventHandler)
	}

	runtime.Goexit()
}
