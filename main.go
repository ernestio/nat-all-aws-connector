/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	ecc "github.com/ernestio/ernest-config-client"
	"github.com/nats-io/nats"
)

var nc *nats.Conn
var natsErr error

func eventHandler(m *nats.Msg) {
	var n Event

	err := n.Process(m.Subject, m.Data)
	if err != nil {
		return
	}

	if err = n.Validate(m.Subject); err != nil {
		n.Error(err)
		return
	}

	parts := strings.Split(m.Subject, ".")
	switch parts[1] {
	case "create":
		err = CreateNat(&n)
	case "update":
		err = UpdateNat(&n)
	case "delete":
		//err = deleteNat(&n)
	}
	if err != nil {
		n.Error(err)
		return
	}

	n.Complete()
}

func internetGatewayByVPCID(svc *ec2.EC2, vpc string) (*ec2.InternetGateway, error) {
	f := []*ec2.Filter{
		&ec2.Filter{
			Name:   aws.String("attachment.vpc-id"),
			Values: []*string{aws.String(vpc)},
		},
	}

	req := ec2.DescribeInternetGatewaysInput{
		Filters: f,
	}

	resp, err := svc.DescribeInternetGateways(&req)
	if err != nil {
		return nil, err
	}

	if len(resp.InternetGateways) == 0 {
		return nil, nil
	}

	return resp.InternetGateways[0], nil
}

func routingTableBySubnetID(svc *ec2.EC2, subnet string) (*ec2.RouteTable, error) {
	f := []*ec2.Filter{
		&ec2.Filter{
			Name:   aws.String("association.subnet-id"),
			Values: []*string{aws.String(subnet)},
		},
	}

	req := ec2.DescribeRouteTablesInput{
		Filters: f,
	}

	resp, err := svc.DescribeRouteTables(&req)
	if err != nil {
		return nil, err
	}

	if len(resp.RouteTables) == 0 {
		return nil, nil
	}

	return resp.RouteTables[0], nil
}

func createInternetGateway(svc *ec2.EC2, vpc string) (string, error) {
	ig, err := internetGatewayByVPCID(svc, vpc)
	if err != nil {
		return "", err
	}

	if ig != nil {
		return *ig.InternetGatewayId, nil
	}

	resp, err := svc.CreateInternetGateway(nil)
	if err != nil {
		return "", err
	}

	req := ec2.AttachInternetGatewayInput{
		InternetGatewayId: resp.InternetGateway.InternetGatewayId,
		VpcId:             aws.String(vpc),
	}

	_, err = svc.AttachInternetGateway(&req)
	if err != nil {
		return "", err
	}

	return *resp.InternetGateway.InternetGatewayId, nil
}

func createRouteTable(svc *ec2.EC2, vpc, subnet string) (*ec2.RouteTable, error) {
	rt, err := routingTableBySubnetID(svc, subnet)
	if err != nil {
		return nil, err
	}

	if rt != nil {
		return rt, nil
	}

	req := ec2.CreateRouteTableInput{
		VpcId: aws.String(vpc),
	}

	resp, err := svc.CreateRouteTable(&req)
	if err != nil {
		return nil, err
	}

	acreq := ec2.AssociateRouteTableInput{
		RouteTableId: resp.RouteTable.RouteTableId,
		SubnetId:     aws.String(subnet),
	}

	_, err = svc.AssociateRouteTable(&acreq)
	if err != nil {
		return nil, err
	}

	return resp.RouteTable, nil
}

func createNatGatewayRoutes(svc *ec2.EC2, rt *ec2.RouteTable, gwID string) error {
	req := ec2.CreateRouteInput{
		RouteTableId:         rt.RouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		NatGatewayId:         aws.String(gwID),
	}

	_, err := svc.CreateRoute(&req)
	if err != nil {
		return err
	}

	return nil
}

// CreateNat ...
func CreateNat(ev *Event) error {
	creds := credentials.NewStaticCredentials(ev.DatacenterAccessKey, ev.DatacenterAccessToken, "")
	svc := ec2.New(session.New(), &aws.Config{
		Region:      aws.String(ev.DatacenterRegion),
		Credentials: creds,
	})

	// Create Elastic IP
	resp, err := svc.AllocateAddress(nil)
	if err != nil {
		return err
	}

	ev.NatGatewayAllocationID = *resp.AllocationId
	ev.NatGatewayAllocationIP = *resp.PublicIp

	// Create Internet Gateway
	ev.InternetGatewayID, err = createInternetGateway(svc, ev.VPCID)
	if err != nil {
		return err
	}

	// Create Nat Gateway
	req := ec2.CreateNatGatewayInput{
		AllocationId: aws.String(ev.NatGatewayAllocationID),
		SubnetId:     aws.String(ev.PublicNetworkAWSID),
	}

	gwresp, err := svc.CreateNatGateway(&req)
	if err != nil {
		return err
	}

	ev.NatGatewayAWSID = *gwresp.NatGateway.NatGatewayId

	waitnat := ec2.DescribeNatGatewaysInput{
		NatGatewayIds: []*string{gwresp.NatGateway.NatGatewayId},
	}

	err = svc.WaitUntilNatGatewayAvailable(&waitnat)
	if err != nil {
		return err
	}

	for _, networkID := range ev.RoutedNetworkAWSIDs {
		rt, err := createRouteTable(svc, ev.VPCID, networkID)
		if err != nil {
			return err
		}

		err = createNatGatewayRoutes(svc, rt, *gwresp.NatGateway.NatGatewayId)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateNat ...
func UpdateNat(ev *Event) error {
	creds := credentials.NewStaticCredentials(ev.DatacenterAccessKey, ev.DatacenterAccessToken, "")
	svc := ec2.New(session.New(), &aws.Config{
		Region:      aws.String(ev.DatacenterRegion),
		Credentials: creds,
	})

	for _, networkID := range ev.RoutedNetworkAWSIDs {
		rt, err := createRouteTable(svc, ev.VPCID, networkID)
		if err != nil {
			return err
		}

		if routeTableIsConfigured(rt, ev.NatGatewayAWSID) {
			continue
		}

		err = createNatGatewayRoutes(svc, rt, ev.NatGatewayAWSID)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteNat ...
func DeleteNat(ev *Event) error {
	creds := credentials.NewStaticCredentials(ev.DatacenterAccessKey, ev.DatacenterAccessToken, "")
	svc := ec2.New(session.New(), &aws.Config{
		Region:      aws.String(ev.DatacenterRegion),
		Credentials: creds,
	})

	req := ec2.DeleteNatGatewayInput{
		NatGatewayId: aws.String(ev.NatGatewayAWSID),
	}

	_, err := svc.DeleteNatGateway(&req)
	if err != nil {
		return err
	}

	for isNatGatewayDeleted(svc, ev.NatGatewayAWSID) == false {
		time.Sleep(time.Second * 3)
	}

	return nil
}

func isNatGatewayDeleted(svc *ec2.EC2, id string) bool {
	gw, _ := natGatewayByID(svc, id)
	if *gw.State == ec2.NatGatewayStateDeleted {
		return true
	}

	return false
}

func routeTableIsConfigured(rt *ec2.RouteTable, gwID string) bool {
	for _, route := range rt.Routes {
		if *route.DestinationCidrBlock == "0.0.0.0/0" && *route.NatGatewayId == gwID {
			return true
		}
	}
	return false
}

func natGatewayByID(svc *ec2.EC2, id string) (*ec2.NatGateway, error) {
	req := ec2.DescribeNatGatewaysInput{
		NatGatewayIds: []*string{aws.String(id)},
	}
	resp, err := svc.DescribeNatGateways(&req)
	if err != nil {
		return nil, err
	}

	if len(resp.NatGateways) != 1 {
		return nil, errors.New("Could not find nat gateway")
	}

	return resp.NatGateways[0], nil
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
