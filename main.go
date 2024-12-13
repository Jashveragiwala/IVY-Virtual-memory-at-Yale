package main

import (
	"bufio"
	"encoding/json"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

const (
	CMPATH     = "centralmanager.json"
	CLIENTPATH = "clients.json"
	Clients    = 1
)

var syscolor = color.New(color.FgCyan).Add(color.BgBlack)
var errcolor = color.New(color.FgHiRed).Add(color.BgBlack)
var warningcolor = color.New(color.FgYellow).Add(color.BgBlack)
var reccolor = color.New(color.FgGreen).Add(color.BgBlack)
var sendcolor = color.New(color.FgHiBlue).Add(color.BgBlack)

// main function
func main() {
	ipAddress := GetOutboundIP().String()
	port, err := GetFreePort()
	if err != nil {
		errcolor.Println("Error assigning port number: ", err)
		return
	}
	portStr := strconv.Itoa(port)
	ipPlusPort := ipAddress + ":" + portStr
	syscolor.Println("IP: ", ipPlusPort)

	reader := bufio.NewReader(os.Stdin)

	var nodeType string
	// Display options to user
	for {
		syscolor.Println("\nInstructions:")
		syscolor.Println("1. Type 1 and Hit ENTER for Central Manager")
		syscolor.Println("2. Type 2 and Hit ENTER for Client")
		syscolor.Println("3. Type 3 and Hit ENTER to Restart Primary Central Manager")
		syscolor.Println("4. Type 4 and Hit ENTER to Restart Backup Central Manager")
		syscolor.Print("\nEnter your choice: ")

		nodeType, err = reader.ReadString('\n')
		if err != nil {
			errcolor.Println("Error reading input: ", err)
			continue
		}
		nodeType = strings.TrimSpace(nodeType)

		switch nodeType {
		case "1":
			StartCM(ipPlusPort)
			return
		case "2":
			StartClient(ipPlusPort)
			return
		case "3":
			RestartPrimaryCM()
			return
		case "4":
			RestartBackupCM()
			return
		default:
			errcolor.Println("Invalid choice. Please try again.")
		}
	}
}

// StartCM starts the Central Manager
func StartCM(IpAddress string) {
	if _, err := os.Stat(CMPATH); os.IsNotExist(err) {
		cm := CentralManager{
			IP:        IpAddress,
			MetaData:  map[string]PgInfo{},
			IsPrimary: true,
		}

		if err := cmwrite([]CentralManager{cm}); err != nil {
			errcolor.Println("Could not write new Central Manager to file: ", err)
			return
		}
		syscolor.Println("Created Central Manager and set as primary: ", cm)

		// Display Central Manager commands
		syscolor.Println("\n--- Available Central Manager Commands ---")
		syscolor.Println("1. data     : Display the current metadata")
		syscolor.Println("   Example: data")
		syscolor.Println("--------------------------------------------\n")

		RunCM(cm)

	} else {
		fileContent, err := os.ReadFile(CMPATH)
		if err != nil {
			errcolor.Println("Could not read from Central Manager's path: ", err)
			return
		}
		var currCM []CentralManager
		if err := json.Unmarshal(fileContent, &currCM); err != nil {
			errcolor.Println(err)
			return
		}
		backupCM := CentralManager{
			IP:        IpAddress,
			IsPrimary: false,
		}
		currCM = append(currCM, backupCM)
		if err := cmwrite(currCM); err != nil {
			errcolor.Println("Could not write to Central Manager's path: ", err)
			return
		}
		syscolor.Println("Created Backup Central Manager: ", backupCM)

		// Display Central Manager commands
		syscolor.Println("\n--- Available Central Manager Commands ---")
		syscolor.Println("1. data     : Display the current metadata")
		syscolor.Println("   Example: data")
		syscolor.Println("--------------------------------------------\n")

		RunCM(backupCM)
	}
}

// RestartPrimaryCM restarts the primary Central Manager
func RestartPrimaryCM() {
	primaryCMIP, err := primaryCMIP()
	if err != nil {
		errcolor.Println("Couldn't get primary Central Manager IP: ", err)
		return
	}
	restartedCM := CentralManager{
		IP:        primaryCMIP,
		IsPrimary: true,
		MetaData:  map[string]PgInfo{},
	}
	allCMs := cmList()
	imBack := Message{
		Type: RECOVERED,
		Payload: Payload{
			Recovered: Recovered{
				CentralManagerIP: restartedCM.IP,
			},
		},
	}

	for _, cm := range allCMs {
		reply := restartedCM.CallRPC(imBack, CENTRALMANAGER, -1, cm.IP)
		if reply.Ack {
			syscolor.Printf("Primary Central Manager with IP: %s is back and taking over\n", restartedCM.IP)
			restartedCM.MetaData = reply.Payload
			syscolor.Println("Data has been restored")
			allClients := clientList()
			for _, client := range allClients {
				changeCM := Message{
					Type: CHANGE_CM,
					Payload: Payload{
						ChangeCM: ChangeCM{
							NewCMIP: restartedCM.IP,
						},
					},
				}
				restartedCM.CallRPC(changeCM, CLIENT, client.ID, client.IP)
			}
		}
	}

	RunCM(restartedCM)
}

// RestartBackupCM restarts the backup Central Manager
func RestartBackupCM() {
	backupCMIP, err := backCMIP()
	if err != nil {
		errcolor.Println("Couldn't get backup Central Manager's IP: ", err)
		return
	}
	restartedBackupCM := CentralManager{
		IP:        backupCMIP,
		IsPrimary: false,
		MetaData:  map[string]PgInfo{},
	}
	RunCM(restartedBackupCM)
}

// StartClient starts the Client
func StartClient(IpAddress string) {
	var client Client
	if _, err := os.Stat(CLIENTPATH); os.IsNotExist(err) {
		cmip, err := primaryCMIP()
		if err != nil {
			errcolor.Println("Couldn't get primary Central Manager's IP: ", err)
			return
		}
		client = Client{
			ID:               1,
			IP:               IpAddress,
			PgCopySet:        make(map[string]Page),
			CentralManagerIP: cmip,
		}
		if err := clientwrite([]Client{client}); err != nil {
			errcolor.Println("Could not write to CLIENTPATH: ", err)
			return
		}
		syscolor.Printf("New Client with ID %d created\n", client.ID)

		// Display Client commands
		syscolor.Println("\n--- Available Client Commands ---")
		syscolor.Println("1. readpg   : Read a specific page")
		syscolor.Println("   Example: readpg P1")
		syscolor.Println("2. writepg  : Write content to a specific page")
		syscolor.Println("   Example: writepg P1 Content1")
		syscolor.Println("3. print    : Display the current Page Copy Set")
		syscolor.Println("4. seed     : Seed pages")
		syscolor.Println("5. run      : Generate 10 random r/w requests and displays the run time")
		syscolor.Println("------------------------------\n")
		RunClient(client)

	} else {
		fileContent, err := os.ReadFile(CLIENTPATH)
		if err != nil {
			errcolor.Println("Could not read from CLIENTPATH: ", err)
			return
		}
		var currClient []Client
		if err := json.Unmarshal(fileContent, &currClient); err != nil {
			errcolor.Println("Error Unmarshalling []Client: ", err)
		}
		highestID := maxClientID(currClient)

		cmip, err := primaryCMIP()
		if err != nil {
			errcolor.Println("Couldn't get primary Central Manager IP: ", err)
			return
		}
		client = Client{
			ID:               highestID + 1,
			IP:               IpAddress,
			PgCopySet:        make(map[string]Page),
			CentralManagerIP: cmip,
		}
		currClient = append(currClient, client)
		if err := clientwrite(currClient); err != nil {
			errcolor.Println("Could not write to CLIENTPATH: ", err)
			return
		}
		syscolor.Printf("New Client with ID %d created\n", client.ID)

		// Display Client commands
		syscolor.Println("\n--- Available Client Commands ---")
		syscolor.Println("1. readpg   : Read a specific page")
		syscolor.Println("   Example: readpg P1")
		syscolor.Println("2. writepg  : Write content to a specific page")
		syscolor.Println("   Example: writepg P1 Content1")
		syscolor.Println("3. print    : Display the current Page Copy Set")
		syscolor.Println("4. seed     : Seed pages")
		syscolor.Println("5. run      : Generate 10 random r/w requests and displays the run time")
		syscolor.Println("------------------------------\n")

		RunClient(client)
	}
}

// RunCM runs the Central Manager
func RunCM(cm CentralManager) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", cm.IP)
	if err != nil {
		errcolor.Println("Error resolving TCP address")
		return
	}
	inbound, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		errcolor.Println("Could not listen to TCP address")
		return
	}
	err = rpc.Register(&cm)
	if err != nil {
		errcolor.Println("Error registering Central Manager's RPC methods: ", err)
		return
	}
	syscolor.Printf("Central Manager's IP: %s\n", cm.IP)
	go rpc.Accept(inbound)

	if !cm.IsPrimary {
		go cm.check()
	}
	reader := bufio.NewReader(os.Stdin)

	for {
		syscolor.Print("Enter Command: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			errcolor.Fprintln(os.Stderr, "Error reading input:", err)
		}
		cm.handleCMInput(strings.TrimSpace(input))
	}
}

// RunClient runs the Client
func RunClient(c Client) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", c.IP)
	if err != nil {
		errcolor.Println("Error resolving TCP address")
	}
	inbound, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		errcolor.Println("Could not listen to TCP address")
	}
	rpc.Register(&c)
	syscolor.Printf("Client%d's IP: %s\n", c.ID, c.IP)
	go rpc.Accept(inbound)
	reader := bufio.NewReader(os.Stdin)

	for {
		syscolor.Print("Enter Command: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			errcolor.Fprintln(os.Stderr, "Error reading input:", err)
		}
		c.handleClientInput(strings.TrimSpace(input))
	}
}

// handleCMInput handles the Central Manager's input
func (cm *CentralManager) handleCMInput(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	userinp := parts[0]
	switch userinp {
	case "data":
		syscolor.Println("MetaData: ", cm.MetaData)
	default:
		syscolor.Println("Wrong Choice")
	}
}

// handleClientInput handles the Client's input
func (c *Client) handleClientInput(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	userinp := parts[0]
	parameters := parts[1:]

	switch userinp {
	// Read a specific page
	case "readpg":
		if len(parameters) != 1 {
			errcolor.Println("Usage: readpg <pageNo>")
			return
		}
		pageNo := parameters[0]
		c.sendReadReq(pageNo)
		// Write content to a specific page
	case "writepg":
		if len(parameters) != 2 {
			errcolor.Println("Usage: writepg <pageNo> <content>")
			return
		}
		pageNo := parameters[0]
		content := parameters[1]
		c.sendWriteReq(pageNo, content)
		// Display the current Page Copy Set
	case "print":
		syscolor.Println("Page Copy Set: ", c.PgCopySet)
		// Seed pages
	case "seed":
		c.seedPg()
	// Generate 10 random r/w requests and displays the run time
	case "run":
		time.Sleep(60 * time.Second)
		start := time.Now().UnixMilli()
		c.reqGenerator()
		end := time.Now().UnixMilli()
		timeTaken := end - start
		syscolor.Printf("Time Taken: %v\n", timeTaken)
	default:
		syscolor.Println("Wrong Choice")
	}

}
