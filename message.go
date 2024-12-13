package main

// Message is a struct that represents a message that is sent between nodes
const (
	READ_REQUEST            = "READ_REQUEST"
	READ_FORWARD            = "READ_FORWARD"
	PAGE_SEND               = "PAGE_SEND"
	READ_CONFIRMATION       = "READ_CONFIRMATION"
	WRITE_REQUEST           = "WRITE_REQUEST"
	INVALIDATE_COPY         = "INVALIDATE_COPY"
	INVALIDATE_CONFIRMATION = "INVALIDATE_CONFIRMATION"
	WRITE_FORWARD           = "WRITE_FORWARD"
	WRITE_CONFIRMATION      = "WRITE_CONFIRMATION"
	PULSE                   = "PULSE"
	CHANGE_CM               = "CHANGE_CM"
	RECOVERED               = "RECOVERED"
)

type Payload struct {
	ReadReq      ReadReq
	ReadForward  ReadForward
	PgSend       PgSend
	ReadConfirm  ReadConfirm
	WriteReq     WriteReq
	InvCopy      InvCopy
	InvConfirm   InvConfirm
	WriteForward WriteForward
	WriteConfirm WriteConfirm
	Pulse        Pulse
	ChangeCM     ChangeCM
	Recovered    Recovered
}

type Message struct {
	Type     string
	Payload  Payload
	SenderID int
	SenderIP string
}

type Reply struct {
	Ack     bool
	Payload map[string]PgInfo
}

type ReadReq struct {
	PgNo string
}

type ReadForward struct {
	ReadReqID int
	ReadReqIP string
	PgNo      string
}

type PgSend struct {
	Purpose string
	Page    Page
}

type ReadConfirm struct {
	PgNum     string
	ReadReqID int
	ReadReqIP string
	SenderID  int
	SenderIP  string
}

type WriteReq struct {
	PgNo    string
	Content string
}

type InvCopy struct {
	WriteReqID int
	PgNum      string
}

type InvConfirm struct {
	WriteReqID int
	PgNum      string
}

type WriteForward struct {
	WriteReqID int
	WriteReqIP string
	PgNum      string
	Content    string
}

type WriteConfirm struct {
	WriterID int
	WriterIP string
	PgNum    string
}

type Pulse struct {
	SenderIP string
}

type ChangeCM struct {
	NewCMIP string
}

type Recovered struct {
	CentralManagerIP string
}
