package mapreduce

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"sync"
)

const (
	sshKeyPath  string = "~/.ssh/mr-key"
	mapperFlag  string = "--mapper"
	reducerFlag string = "--reducer"
	configFlag  string = "--config"
)

type Server struct {
	address string
	host    string
	cwd     string

	listener          net.Listener
	verbose           bool
	mapOnly           bool
	numMappers        int
	numReducers       int
	mapper            string //mr executable
	reducer           string //mrm executable
	inputDir          string
	intermediateDir   string
	outputDir         string
	mapperExecutable  string //wordcount-mapper
	reducerExecutable string //wordcount-reducer

	nodes           []string
	ipAddressMap    map[string]string
	serverIsRunning bool

	unprocessed []string
	inflight    map[string]bool
}

func NewServer(args []string) *Server {
	s := Server{}
	s.host = GetHost()
	s.cwd = GetCwd()
	s.verbose = true
	s.mapOnly = false
	s.serverIsRunning = false
	s.inflight = make(map[string]bool)
	s.address = ":8000"

	s.getNodes()
	s.buildIPAddrMap()

	s.parseArgumentList(args)
	s.stageInputFiles()
	s.startServer()

	return &s
}

func (s *Server) Run() {
	s.spawnMappers()
	if !s.mapOnly {
		s.spawnReducers()
	}
	s.shutDown()
}

func (s *Server) startServer() {
	ln, err := net.Listen("tcp", s.address)
	if err != nil {
		log.Fatal(MapReduceError{errStartingServer, err})
	}

	log.Println("Server listening on: ", ExternalServerAddress)
	s.listener = ln
	go s.orchestrateWorkers()
}

func (s *Server) orchestrateWorkers() {
	s.serverIsRunning = true
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Fatal(MapReduceError{errCouldNotConnect, err})
		}
		defer conn.Close()

		if !s.serverIsRunning {
			s.listener.Close()
		}
		fmt.Print("\n")

		remoteIPAddr := getIPAddrFromConn(conn)
		log.Println("Received connection request from: ", s.ipAddressMap[remoteIPAddr])
		s.handleRequest(conn, remoteIPAddr)
	}
}

func (s *Server) handleRequest(conn net.Conn, remoteIPAddr string) {
	msgString := readFromConn(conn)
	msg := extractMessageFromString(msgString)
	log.Println("Received worker message: ", msg)

	if msg == WorkerReady {
		path := s.getUnprocessedFilePattern() //*hidden side-effect*
		if !isEmpty(path) {
			sendMessage(conn, path, "")
		} else {
			sendMessage(conn, ServerDone.toString(), "")
		}
	} else if msg == JobSucceeded {
		filePattern := extractValueFromString(msgString)
		s.markFilePatternAsProcessed(remoteIPAddr, filePattern)
	} else if msg == JobFailed {
		filePattern := extractValueFromString(msgString)
		s.rescheduleFilePatternJob(remoteIPAddr, filePattern)
	} else if msg == JobInfo {
		info := extractValueFromString(msgString)
		log.Println("Received job info:", info)
	} else {
		log.Println("Ignoring unrecognized message:", msgString)
	}
}

func (s *Server) spawnMappers() {
	var wg sync.WaitGroup
	wg.Add(s.numMappers)
	for i := 0; i < s.numMappers; i++ {
		mapperNode := s.getRandomNode()
		command := s.buildMapperCommand(mapperNode)
		log.Println("Starting mapper command on remote machine:", mapperNode)
		go s.spawnWorker(command, &wg)
	}
	wg.Wait()
	PrintStageMessage("MAPPERS FINISHED")
}

func (s *Server) buildMapperCommand(remoteMachine string) *exec.Cmd {
	numHashCodes := s.numMappers * s.numReducers
	commandFlag := fmt.Sprintf("--command=sudo %s %s %s %d", s.mapper, s.mapperExecutable, s.intermediateDir, numHashCodes)
	zoneFlag := fmt.Sprintf("--zone=%s", ServerZone)
	command := exec.Command("gcloud", "compute", "ssh", remoteMachine, zoneFlag, commandFlag)
	return command
}

func (s *Server) spawnReducers() {
	s.fillUnprocessedWithIntermediates()

	var wg sync.WaitGroup
	wg.Add(s.numReducers)
	for i := 0; i < s.numReducers; i++ {
		reducerNode := s.getRandomNode()
		command := s.buildReducerCommand(reducerNode)
		log.Println("Starting reducer command on remote machine:", reducerNode)
		go s.spawnWorker(command, &wg)
	}
	wg.Wait()
	PrintStageMessage("REDUCERS FINISHED")
}

func (s *Server) fillUnprocessedWithIntermediates() {
	s.unprocessed = nil
	for i := 0; i < s.numMappers*s.numReducers; i++ {
		filePattern := s.intermediateDir + "*." + GetPaddedNumber(i) + ".mapped"
		s.unprocessed = append(s.unprocessed, filePattern)
	}
}

func (s *Server) buildReducerCommand(remoteMachine string) *exec.Cmd {
	commandFlag := fmt.Sprintf("--command=sudo %s %s %s", s.reducer, s.reducerExecutable, s.outputDir)
	zoneFlag := fmt.Sprintf("--zone=%s", ServerZone)
	command := exec.Command("gcloud", "compute", "ssh", remoteMachine, zoneFlag, commandFlag)
	return command
}

func (s *Server) spawnWorker(command *exec.Cmd, wg *sync.WaitGroup) {
	err := command.Start()
	if err != nil {
		log.Fatal(MapReduceError{errExecutingCmd, err})
	}
	err = command.Wait()
	log.Println("Worker command exited with status:", err)
	wg.Done()
}

func (s *Server) shutDown() {
	s.serverIsRunning = false
	s.listener.Close()
}
