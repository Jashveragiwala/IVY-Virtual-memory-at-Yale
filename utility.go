package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
)

const (
	CLIENT         = "Client"
	CENTRALMANAGER = "CentralManager"
)

func (cm *CentralManager) CallRPC(msg Message, nodeType string, targetID int, targetIP string) (reply Reply) {
	sendcolor.Printf("Central Manager with is sending Msg '%s' to Client%d\n", removeUnderscores(msg.Type), targetID)
	clnt, err := rpc.Dial("tcp", targetIP)
	if err != nil {
		errcolor.Println("Error dialing RPC: ", err)
		reply.Ack = false
		return reply
	}
	err = clnt.Call(fmt.Sprintf("%s.HandleIncMsg", nodeType), msg, &reply)
	if err != nil {
		errcolor.Printf("Error calling RPC from Msg '%s': %v\n", removeUnderscores(msg.Type), err)
		reply.Ack = false
		return reply
	}
	return reply
}

// CallRPC is a method for Client struct that sends a message to a target node
func (client *Client) CallRPC(msg Message, nodeType string, targetID int, targetIP string) (reply Reply) {
	sendcolor.Printf("Client%d is sending Msg '%s' to %s%d\n", client.ID, removeUnderscores(msg.Type), nodeType, targetID)
	clnt, err := rpc.Dial("tcp", targetIP)
	if err != nil {
		errcolor.Println("Error dialing RPC: ", err)
		reply.Ack = false
		return reply
	}
	err = clnt.Call(fmt.Sprintf("%s.HandleIncMsg", nodeType), msg, &reply)
	if err != nil {
		errcolor.Printf("Error calling RPC from Msg '%s': %v\n", removeUnderscores(msg.Type), err)
		reply.Ack = false
		return reply
	}
	return reply
}

// GetOutboundIP gets the IP address of the current machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}

// GetFreePort gets a free port for the current machine
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// writes the central manager's IP to a file
func cmwrite(cms []CentralManager) error {
	cmJSON, err := json.MarshalIndent(cms, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(CMPATH, cmJSON, 0644)
	if err != nil {
		return err
	}
	return nil
}

// writes the client's IP to a file
func clientwrite(clients []Client) error {
	clientJSON, err := json.MarshalIndent(clients, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(CLIENTPATH, clientJSON, 0644)
	if err != nil {
		return err
	}
	return nil
}

func primaryCMIP() (string, error) {
	fileContent, err := os.ReadFile(CMPATH)
	if err != nil {
		errcolor.Println("Error reading centralmanager.json: ", err)
		return "NIL", err
	}
	var cms []CentralManager
	if err := json.Unmarshal(fileContent, &cms); err != nil {
		errcolor.Println("Error Unmarshalling []Central Manager: ", err)
	}
	for _, cm := range cms {
		if cm.IsPrimary {
			return cm.IP, nil
		}
	}
	errcolor.Println("Primary Central Manager not found: ", err)
	return "NIL", err
}

func backCMIP() (string, error) {
	fileContent, err := os.ReadFile(CMPATH)
	if err != nil {
		errcolor.Println("Error reading cm.json: ", err)
		return "NIL", err
	}
	var cms []CentralManager
	if err := json.Unmarshal(fileContent, &cms); err != nil {
		errcolor.Println("Error Unmarshalling []Central Manager: ", err)
	}
	for _, cm := range cms {
		if !cm.IsPrimary {
			return cm.IP, nil
		}
	}
	errcolor.Println("Primary Central Manager not found: ", err)
	return "NIL", err
}

// maxClientID returns the maximum client ID in the list of clients
func maxClientID(clients []Client) int {
	if len(clients) == 0 {
		return -1
	}
	ID := 0
	for _, client := range clients {
		if client.ID > ID {
			ID = client.ID
		}
	}
	return ID
}

func clientList() []Client {
	fileContent, err := os.ReadFile(CLIENTPATH)
	if err != nil {
		errcolor.Println("Error reading client.json: ", err)
		return []Client{}
	}
	var list []Client
	if err := json.Unmarshal(fileContent, &list); err != nil {
		errcolor.Println("Error Unmarshalling []Client: ", err)
	}
	return list
}

// cmList returns a list of central managers
func cmList() []CentralManager {
	fileContent, err := os.ReadFile(CMPATH)
	if err != nil {
		errcolor.Println("Error reading centralmanager.json: ", err)
		return []CentralManager{}
	}
	var list2 []CentralManager
	if err := json.Unmarshal(fileContent, &list2); err != nil {
		errcolor.Println("Error Unmarshalling []Central Manager: ", err)
	}
	return list2
}
