package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const (
	READ      = "READ"
	WRITE     = "WRITE"
	READWRITE = "READWRITE"
	NIL       = "NIL"
)

type Page struct {
	PageId  string
	Content string
	Access  string
}

type Client struct {
	ID               int
	IP               string
	PgCopySet        map[string]Page
	CentralManagerIP string
}

type ClientPointer struct {
	ID int
	IP string
}

// Utility function to remove underscores
func removeUnderscores(s string) string {
	return strings.ReplaceAll(s, "_", " ")
}

// HandleIncMsg handles incoming messages
func (c *Client) HandleIncMsg(msg Message, reply *Reply) error {
	reccolor.Printf("Message of Type '%s' received\n", removeUnderscores(msg.Type))
	switch msg.Type {
	case READ_FORWARD:
		c.HandleReadFrd(msg)
		reply.Ack = true
	case PAGE_SEND:
		c.HandlePgSend(msg)
		reply.Ack = true
	case INVALIDATE_COPY:
		reply.Ack = c.handleInvalidate(msg)
	case WRITE_FORWARD:
		c.handleWriteForward(msg)
		reply.Ack = true
	case CHANGE_CM:
		c.handleChangeCentralManager(msg)
		reply.Ack = true
	}
	return nil
}

// HandleReadFrd handles a READ_FORWARD message
func (c *Client) HandleReadFrd(msg Message) {
	reqPgNo := msg.Payload.ReadForward.PgNo
	reqPg := c.PgCopySet[reqPgNo]
	pgSendMsg := Message{
		Type: PAGE_SEND,
		Payload: Payload{
			PgSend: PgSend{
				Purpose: READ,
				Page:    reqPg,
			}},
		SenderID: c.ID,
		SenderIP: c.IP,
	}
	readReqID := msg.Payload.ReadForward.ReadReqID
	readReqIP := msg.Payload.ReadForward.ReadReqIP
	sendcolor.Printf("Client %d sending Msg '%s' to Client %d\n", c.ID, removeUnderscores(PAGE_SEND), readReqID)
	reply := c.CallRPC(pgSendMsg, CLIENT, readReqID, readReqIP)
	if !reply.Ack {
		errcolor.Printf("Msg '%s' from Client %d not acknowledged by Client %d\n", removeUnderscores(pgSendMsg.Type), c.ID, readReqID)
		return
	}
}

// HandlePgSend handles a PAGE_SEND message
func (c *Client) HandlePgSend(msg Message) {
	sentPgNo := msg.Payload.PgSend.Page.PageId
	sentPg := msg.Payload.PgSend.Page
	why := msg.Payload.PgSend.Purpose

	if why == READ {
		sentPg.Access = READ
		readConf := Message{
			Type: READ_CONFIRMATION,
			Payload: Payload{
				ReadConfirm: ReadConfirm{
					PgNum:     sentPgNo,
					ReadReqID: c.ID,
					ReadReqIP: c.IP,
					SenderID:  msg.SenderID,
					SenderIP:  msg.SenderIP,
				},
			},
		}
		reply := c.CallRPC(readConf, CENTRALMANAGER, -1, c.CentralManagerIP)
		if !reply.Ack {
			errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", c.ID, removeUnderscores(READ_CONFIRMATION))
			return
		}

	} else if why == WRITE {
		sentPg.Access = READWRITE
		writeConf := Message{
			Type: WRITE_CONFIRMATION,
			Payload: Payload{
				WriteConfirm: WriteConfirm{
					PgNum:    sentPgNo,
					WriterID: c.ID,
					WriterIP: c.IP,
				},
			},
		}
		reply := c.CallRPC(writeConf, CENTRALMANAGER, -1, c.CentralManagerIP)
		if !reply.Ack {
			errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", c.ID, removeUnderscores(WRITE_CONFIRMATION))
			return
		}
	}
	c.PgCopySet[sentPgNo] = sentPg
}

// handles an INVALIDATE_COPY message
func (c *Client) handleInvalidate(msg Message) bool {
	targetPageNo := msg.Payload.InvCopy.PgNum
	targetPage, exists := c.PgCopySet[targetPageNo]
	if !exists {
		errcolor.Printf("Page %s doesn't exist in Node %d's PgCopySet. Cannot invalidate", targetPageNo, c.ID)
		return false
	}
	targetPage.Access = NIL
	c.PgCopySet[targetPageNo] = targetPage
	return true
}

// handles a WRITE_FORWARD message
func (c *Client) handleWriteForward(msg Message) {
	writeReqID := msg.Payload.WriteForward.WriteReqID
	writeReqIP := msg.Payload.WriteForward.WriteReqIP
	ReqPg := msg.Payload.WriteForward.PgNum
	content := msg.Payload.WriteForward.Content
	page, exists := c.PgCopySet[ReqPg]
	page.Access = NIL
	page.Content = content
	c.PgCopySet[ReqPg] = page

	if !exists {
		errcolor.Printf("Client %d req to write Page %s does not exist in Client %d's PgCopySet", writeReqID, ReqPg, c.ID)
		return
	}
	pageSend := Message{
		Type: PAGE_SEND,
		Payload: Payload{
			PgSend: PgSend{
				Purpose: WRITE,
				Page:    page,
			},
		},
		SenderID: c.ID,
		SenderIP: c.IP,
	}
	sendcolor.Printf("Client %d sending Msg %s to Client %d\n", c.ID, removeUnderscores(PAGE_SEND), writeReqID)
	reply := c.CallRPC(pageSend, CLIENT, writeReqID, writeReqIP)
	if !reply.Ack {
		errcolor.Printf("Msg '%s' from Client %d not acknowledged by Client %d\n", removeUnderscores(INVALIDATE_CONFIRMATION), c.ID, writeReqID)
		return
	}
}

// sends a READ_REQUEST message
func (c *Client) sendReadReq(pageNo string) {
	readRequest := Message{
		Type: READ_REQUEST,
		Payload: Payload{
			ReadReq: ReadReq{
				PgNo: pageNo,
			},
		},
		SenderID: c.ID,
		SenderIP: c.IP,
	}

	reply := c.CallRPC(readRequest, CENTRALMANAGER, -1, c.CentralManagerIP)
	if !reply.Ack {
		errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", c.ID, removeUnderscores(READ_REQUEST))
	}
}

// sends a WRITE_REQUEST message
func (c *Client) sendWriteReq(pageNo string, content string) {
	page, exists := c.PgCopySet[pageNo]
	// If the page already exists
	if exists {
		// If the page is already stored in the Central Manager
		if page.Access == READWRITE {
			syscolor.Printf("You already have %s access to page %s\n", page.Access, pageNo)
			page.Content = content
			c.PgCopySet[pageNo] = page
			return
		} else {
			syscolor.Printf("Page %s exists and you have %s access\n", pageNo, page.Access)
		}
	} else {
		syscolor.Printf("Page %s does not exist but you can create one\n", pageNo)
	}
	writeRequest := Message{
		Type: WRITE_REQUEST,
		Payload: Payload{
			WriteReq: WriteReq{
				PgNo:    pageNo,
				Content: content,
			},
		},
		SenderID: c.ID,
		SenderIP: c.IP,
	}

	reply := c.CallRPC(writeRequest, CENTRALMANAGER, -1, c.CentralManagerIP)
	if !reply.Ack {
		errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", c.ID, removeUnderscores(READ_REQUEST))
	}
}

// handles a CHANGE_CM message
func (c *Client) handleChangeCentralManager(msg Message) {
	c.CentralManagerIP = msg.Payload.ChangeCM.NewCMIP
	syscolor.Printf("Changed CentralManagerIP to %s\n", c.CentralManagerIP)
}

func (c *Client) seedPg() {
	for i := 1; i <= 10; i++ {
		c.sendWriteReq(fmt.Sprintf("P%d", i), fmt.Sprintf("Content by Client %d", c.ID))
	}
}

// geenrates random read and write requests
func (c *Client) reqGenerator() {
	for i := 0; i < Clients; i++ {
		time.Sleep(1 * time.Second)
		n := rand.Intn(2)
		if n == 0 {
			c.sendWriteReq(fmt.Sprintf("P%d", rand.Intn(10)), fmt.Sprintf("Content by Client %d", c.ID))
		} else {
			c.sendReadReq(fmt.Sprintf("P%d", rand.Intn(10)))
		}
	}
}

// 90% read - 10% write
// func (c *Client) reqGenerator() {
// 	for i := 0; i < Clients; i++ {
// 		time.Sleep(1 * time.Second)
// 		n := rand.Intn(10)
// 		if n == 0 {
// 			c.sendWriteReq(fmt.Sprintf("P%d", rand.Intn(10)), fmt.Sprintf("Content by Client %d", c.ID))
// 		} else {
// 			c.sendReadReq(fmt.Sprintf("P%d", rand.Intn(10)))
// 		}
// 	}
// }

// 10% read - 90% write
// func (c *Client) reqGenerator() {
// 	for i := 0; i < Clients; i++ {
// 		time.Sleep(1 * time.Second)
// 		n := rand.Intn(10)
// 		if n == 0 {
// 			c.sendReadReq(fmt.Sprintf("P%d", rand.Intn(10)))
// 		} else {
// 			c.sendWriteReq(fmt.Sprintf("P%d", rand.Intn(10)), fmt.Sprintf("Content by Client %d", c.ID))
// 		}
// 	}
// }
