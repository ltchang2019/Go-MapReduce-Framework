package mapreduce

import (
	"log"
	"net"
	"strings"
)

type Message string

const (
	WorkerReady    Message = "WORKER_READY"
	JobStarted     Message = "JOB_STARTED"
	JobInfo        Message = "JOB_INFO"
	JobSucceeded   Message = "JOB_SUCCEEDED"
	JobFailed      Message = "JOB_FAILED"
	ServerDone     Message = "SERVER_DONE"
	UnknownMessage Message = "UNKNOWN_MESSAGE"
)

var stringToMessageMap = map[string]Message{
	"WORKER_READY":  WorkerReady,
	"JOB_STARTED":   JobStarted,
	"JOB_INFO":      JobInfo,
	"JOB_SUCCEEDED": JobSucceeded,
	"JOB_FAILED":    JobFailed,
	"SERVER_DONE":   ServerDone,
}

var messageToStringMap = map[Message]string{
	WorkerReady:  "WORKER_READY",
	JobStarted:   "JOB_STARTED",
	JobInfo:      "JOB_INFO",
	JobSucceeded: "JOB_SUCCEEDED",
	JobFailed:    "JOB_FAILED",
	ServerDone:   "SERVER_DONE",
}

func extractMessageFromString(msgString string) Message {
	key := strings.Split(msgString, " ")[0]
	msg, exists := stringToMessageMap[key]
	if !exists {
		return UnknownMessage
	}
	return msg
}

func extractValueFromString(msgString string) string {
	valueIndex := strings.Index(msgString, " ")
	return msgString[valueIndex+1:]
}

func readFromConn(conn net.Conn) string {
	buf := make([]byte, 1024)
	strLen, err := conn.Read(buf)
	if err != nil {
		log.Fatal(MapReduceError{errReadingMessage, err.Error()})
	}
	return string(buf[:strLen])
}

func (s *Server) sendJobStart(conn net.Conn, path string) {
	_, err := conn.Write([]byte(path))
	if err != nil {
		log.Fatal(MapReduceError{errWritingMessage, err.Error()})
	}
}

func (s *Server) sendServerDone(conn net.Conn) {
	_, err := conn.Write([]byte(ServerDone))
	if err != nil {
		log.Fatal(MapReduceError{errWritingMessage, err.Error()})
	}
}

func (w *Worker) sendWorkerReady(conn net.Conn) {
	_, err := conn.Write([]byte(WorkerReady))
	if err != nil {
		log.Fatal(MapReduceError{errWritingMessage, err.Error()})
	}
}

func (w *Worker) sendJobProgressAlert(conn net.Conn, info string) {
	_, err := conn.Write([]byte(messageToStringMap[JobInfo] + " " + info))
	if err != nil {
		log.Fatal(MapReduceError{errWritingMessage, err.Error()})
	}
}

func (w *Worker) sendJobSucceeded(conn net.Conn, fileName string) {
	_, err := conn.Write([]byte(messageToStringMap[JobSucceeded] + " " + fileName))
	if err != nil {
		log.Fatal(MapReduceError{errWritingMessage, err.Error()})
	}
}

func (w *Worker) sendJobFailed(conn net.Conn, fileName string) {
	_, err := conn.Write([]byte(messageToStringMap[JobFailed] + " " + fileName))
	if err != nil {
		log.Fatal(MapReduceError{errWritingMessage, err.Error()})
	}
}
