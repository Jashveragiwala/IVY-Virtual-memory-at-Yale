package main

import (
	"time"
)

// CentralManager is a struct that represents a Central Manager node
type CentralManager struct {
	IP        string
	MetaData  map[string]PgInfo
	IsPrimary bool
}

// PgInfo is a struct that represents the information of a page
type PgInfo struct {
	Owner   ClientPointer
	CopySet []ClientPointer
}

// HandleIncMsg handles incoming messages
func (cm *CentralManager) HandleIncMsg(msg Message, reply *Reply) error {
	reccolor.Printf("Message of Type '%s' received\n", removeUnderscores(msg.Type))
	if cm.IsPrimary {
		switch msg.Type {
		case READ_REQUEST:
			cm.handleReadReq(msg)
			reply.Ack = true
		case READ_CONFIRMATION:
			cm.handleReadConfirmation(msg)
			reply.Ack = true
		case WRITE_REQUEST:
			cm.handleWriteReq(msg)
			reply.Ack = true
		case WRITE_CONFIRMATION:
			cm.handleWriteConfirmation(msg)
			reply.Ack = true
		case PULSE:
			reply.Payload = cm.MetaData
			reply.Ack = true
		case RECOVERED:
			cm.IsPrimary = false
			reply.Payload = cm.MetaData
			reply.Ack = true
			go cm.check()
		}
	}

	return nil
}

// Handles a READ_REQUEST message
func (cm *CentralManager) handleReadReq(msg Message) {
	pgNo := msg.Payload.ReadReq.PgNo
	page, exists := cm.MetaData[pgNo]
	if !exists {
		errcolor.Printf("Central Manager doesn't have Page %s\n", pgNo)
		errcolor.Printf("ReadReq by Client %d denied\n", msg.SenderID)
		return
	}
	pgOwner := page.Owner
	readForward := Message{
		Type: READ_FORWARD,
		Payload: Payload{
			ReadForward: ReadForward{
				ReadReqID: msg.SenderID,
				ReadReqIP: msg.SenderIP,
				PgNo:      msg.Payload.ReadReq.PgNo,
			}},
	}

	sendcolor.Printf("Central Manager sending Msg '%s' to Client %d\n", removeUnderscores(READ_FORWARD), pgOwner.ID)
	reply := cm.CallRPC(readForward, CLIENT, pgOwner.ID, pgOwner.IP)
	if !reply.Ack {
		errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", pgOwner.ID, removeUnderscores(readForward.Type))
		return
	}
}

// Handles a READ_CONFIRMATION message
func (cm *CentralManager) handleReadConfirmation(msg Message) {
	reqPg := msg.Payload.ReadConfirm.PgNum
	readReqID := msg.Payload.ReadConfirm.ReadReqID
	readReqIP := msg.Payload.ReadConfirm.ReadReqIP
	pgOwner := cm.MetaData[reqPg].Owner
	reqPointer := ClientPointer{ID: readReqID, IP: readReqIP}
	updatedCopySet := append(cm.MetaData[reqPg].CopySet, reqPointer)
	cm.MetaData[reqPg] = PgInfo{Owner: pgOwner, CopySet: updatedCopySet}
	syscolor.Println("Updated Copyset: ", updatedCopySet)
}

// handleWriteReq handles a WRITE_REQUEST message
func (cm *CentralManager) handleWriteReq(msg Message) {
	targetPg := msg.Payload.WriteReq.PgNo
	content := msg.Payload.WriteReq.Content
	writeReqID := msg.SenderID
	writeReqIP := msg.SenderIP
	writeReqPointer := ClientPointer{
		ID: writeReqID,
		IP: writeReqIP,
	}

	pgInfo, exists := cm.MetaData[targetPg]
	if !exists {
		warningcolor.Printf("Central Manager doesn't have Page %s stored\n", targetPg)
		warningcolor.Printf("Creating and Adding Page %s into Central Manager's record\n", targetPg)
		cm.MetaData[targetPg] = PgInfo{
			Owner:   writeReqPointer,
			CopySet: []ClientPointer{},
		}
		syscolor.Printf("PgInfo stored:%v\n", cm.MetaData[targetPg])
		pageSend := Message{
			Type: PAGE_SEND,
			Payload: Payload{
				PgSend: PgSend{
					Purpose: WRITE,
					Page: Page{
						PageId:  targetPg,
						Content: content,
					},
				},
			},
		}
		reply := cm.CallRPC(pageSend, CLIENT, writeReqID, writeReqIP)
		if !reply.Ack {
			errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", writeReqID, removeUnderscores(PAGE_SEND))
		}
		return
	}
	// If the page is already stored in the Central Manager
	for _, clientPointer := range pgInfo.CopySet {
		invalidateCopy := Message{
			Type: INVALIDATE_COPY,
			Payload: Payload{
				InvCopy: InvCopy{
					WriteReqID: writeReqID,
					PgNum:      targetPg,
				},
			},
		}

		reply := cm.CallRPC(invalidateCopy, CLIENT, clientPointer.ID, clientPointer.IP)
		if !reply.Ack {
			errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", clientPointer.ID, removeUnderscores(invalidateCopy.Type))
			errcolor.Println("Central Manager was unable to forward Write Request")
			return
		}
	}

	writeForward := Message{
		Type: WRITE_FORWARD,
		Payload: Payload{
			WriteForward: WriteForward{
				WriteReqID: writeReqID,
				WriteReqIP: writeReqIP,
				PgNum:      targetPg,
				Content:    content,
			},
		},
	}
	updatedPgInfo := cm.MetaData[targetPg]
	ownerID := updatedPgInfo.Owner.ID
	ownerIP := updatedPgInfo.Owner.IP
	reply := cm.CallRPC(writeForward, CLIENT, ownerID, ownerIP)
	if !reply.Ack {
		errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", ownerID, removeUnderscores(writeForward.Type))
		return
	}
}

// handleWriteConfirmation handles a WRITE_CONFIRMATION message
func (cm *CentralManager) handleWriteConfirmation(msg Message) {
	newPgNo := msg.Payload.WriteConfirm.PgNum
	newPg, exists := cm.MetaData[newPgNo]
	if !exists {
		errcolor.Printf("Central Manager doesn't have Page %s Info stored", newPgNo)
		return
	}
	writerID := msg.Payload.WriteConfirm.WriterID
	writerIP := msg.Payload.WriteConfirm.WriterIP
	newPg.Owner = ClientPointer{ID: writerID, IP: writerIP}
	newPg.CopySet = []ClientPointer{}
	cm.MetaData[newPgNo] = newPg
}

// check checks if the Primary Central Manager is alive
func (cm *CentralManager) check() {
	for {
		time.Sleep(2 * time.Second)
		pulse := Message{
			Type: PULSE,
			Payload: Payload{
				Pulse: Pulse{
					SenderIP: cm.IP,
				},
			},
		}
		primaryCMIP, err := primaryCMIP()
		if err != nil {
			errcolor.Println("Backup Central Manager couldn't get Primary Central Manager's IP")
			return
		}
		reply := cm.CallRPC(pulse, CENTRALMANAGER, -1, primaryCMIP)
		if !reply.Ack {
			errcolor.Println("PULSE not retrived from the Primary Central Manager")
			errcolor.Println("Primary Central Manager is dead")
			syscolor.Println("Backup Central Manager is taking over Now")
			cm.IsPrimary = true
			syscolor.Println("Backup Central Manager is Primary Central Manager now")

			clientArr := clientList()
			// Change the Central Manager IP in all the clients
			for _, client := range clientArr {
				changeCM := Message{
					Type: CHANGE_CM,
					Payload: Payload{
						ChangeCM: ChangeCM{
							NewCMIP: cm.IP,
						},
					},
				}
				reply := cm.CallRPC(changeCM, CLIENT, client.ID, client.IP)
				if !reply.Ack {
					errcolor.Printf("Central Manager did not acknowledge Client %d's Msg '%s'\n", client.ID, removeUnderscores(CHANGE_CM))
					return
				}
			}
			return
		} else {
			cm.MetaData = reply.Payload
		}
	}
}
