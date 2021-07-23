package main

import (
	"air-hockey-backend/pb"
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"sync"
)

var lock = &sync.RWMutex{}
var players = make(map[string]*Player)
var rooms = make(map[string]*Room)

type server struct{}

type Room struct {
	host			string
	ID				uuid.UUID
	roomPlayers 	[]string
	maxPlayer		int32
	maxScore		int32
	WaitGroup 		*sync.WaitGroup
}

type Player struct {
	name      		string
	channel 		chan pb.GameMessage
	uuid 			string
	WaitGroup 		*sync.WaitGroup
}


func main() {
	port := ":50069"
	lis, err := net.Listen("tcp", port)

	if err != nil {
		log.Fatalf("Failed to listen %v", err)
	}
	log.Println("Server opened port 50069")
	// Initializes the gRPC server.
	s := grpc.NewServer()

	// Register the server with gRPC.
	pb.RegisterAirHockeyServiceServer(s, &server{})

	// Register reflection service on gRPC server.
	reflection.Register(s)
	log.Println("Server running")

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func (s *server) Connect(_ context.Context, in *pb.NewPlayerName) (*pb.PlayerInfo, error) {
	name := in.Name
	id := uuid.New()
	AddPlayer(id, name)
	return &pb.PlayerInfo{Name: name, Uuid: id.String()}, nil
}

func AddPlayer(id uuid.UUID, name string) {
	lock.Lock()
	defer lock.Unlock()
	newPlayer := &Player{
		name:      			name,
		channel: 		 	make(chan pb.GameMessage, 100),
		WaitGroup:			&sync.WaitGroup{},
		uuid: 				id.String(),
	}
	log.Print("[AddClient]: Registered player: " + name)
	players[id.String()] = newPlayer
}

func (s *server) NewRoom(_ context.Context, in *pb.NewGameInfo) (*pb.RoomID, error) {
	lock.Lock()
	defer lock.Unlock()

	newRoom := &Room{
		host: 			in.Host,
		ID:      		uuid.New(),
		maxPlayer: 		in.NumberOfPlayer,
		maxScore: 		in.TargetScore,
		WaitGroup: 		&sync.WaitGroup{},
	}
	newRoomID := newRoom.ID.String()
	log.Print("[AddRoom]: with ID " + newRoom.ID.String())
	rooms[newRoomID] = newRoom
	rooms[newRoomID].roomPlayers = append(rooms[newRoomID].roomPlayers, in.Host)
	rooms[newRoomID].WaitGroup.Add(1)
	return &pb.RoomID{UniqueID: newRoomID}, nil
}

func (s *server) JoinRoom(_ context.Context, in *pb.JoinRequest) (*pb.RoomID, error){
	roomID := in.RoomID
	if roomID == ""{
		for _, singleRoom := range rooms{
			if len(singleRoom.roomPlayers) != int(singleRoom.maxPlayer) {
				singleRoom.roomPlayers = append(singleRoom.roomPlayers, in.PlayerInfo.Uuid)
				roomID = singleRoom.ID.String()
			}
			break
		}
		return &pb.RoomID{UniqueID: roomID}, nil
	}

	if RoomExists(in.RoomID ) {
		AddPlayerToRoom(in.PlayerInfo.Uuid, in.RoomID)
		return &pb.RoomID{UniqueID: in.RoomID}, nil
	}

	return nil, errors.New("room does not exist")
}

func RoomExists(roomID string) bool {
	lock.RLock()
	defer lock.RUnlock()
	for _, singleRoom := range rooms {
		if singleRoom.ID.String() == roomID {
			return true
		}
	}
	return false
}

func AddPlayerToRoom(playerID string, roomID string)  {
	rooms[roomID].WaitGroup.Add(1)
	defer rooms[roomID].WaitGroup.Done()
	rooms[roomID].roomPlayers = append(rooms[roomID].roomPlayers, playerID)
}

func (s *server) GetPlayerList(context.Context, *pb.Empty) (*pb.PlayerList, error) {
	var c []string
	for key := range players {
		c = append(c, key)
	}

	log.Print("[GetClientList]: Returned list of current Clients ")
	log.Print(c)

	return &pb.PlayerList{Players: c}, nil
}

func (s *server) GameStream(svr pb.AirHockeyService_GameStreamServer) error {
	req, err := svr.Recv()

	if err != nil {
		return err
	}
	//hostID := GetHost(msg.RoomID)
	outbox := make(chan pb.GameMessage, 100)
	go ListenToClient(svr, outbox)

	for {
		select {
		case outMsg := <-outbox:
			switch outMsg.GetAction().(type) {
			case *pb.GameMessage_GameState :
				BroadcastGameState(outMsg.GetGameState().RoomID, outMsg)
			case *pb.GameMessage_EntityState:
				BroadcastToPlayer(outMsg.GetEntityState().RoomID, outMsg)
				log.Print("having ENTITY")
			case *pb.GameMessage_PlayerInput:
				log.Print("Having INPUT")
				hostID := GetHost(outMsg)
				BroadcastToHost(hostID, outMsg)
			case *pb.GameMessage_Empty:
				//BroadcastGameState(outMsg.GetGameState().RoomID, outMsg)
			}
		case inMsg := <-players[req.Sender].channel:
			log.Print(players[req.Sender].channel)
			err := svr.Send(&inMsg)
			if err != nil {
				return err
			}
		}
	}
}

func GetHost(playerInput pb.GameMessage) string{
	roomID := playerInput.GetPlayerInput().RoomID
	for _, singleRoom := range rooms{
		if singleRoom.ID.String() == roomID{
			return singleRoom.host
		}
	}
	return "Wrong room ID"
}

func BroadcastToHost(hostID string, msg pb.GameMessage){
	lock.Lock()
	defer lock.Unlock()
	players[hostID].channel <- msg
}

func ListenToClient(svr pb.AirHockeyService_GameStreamServer, messages chan<- pb.GameMessage) {
	for {
		req, err := svr.Recv()
		if err != nil{
			log.Fatal(err)
		}
		messages <- *req
	}
}

func BroadcastToPlayer(roomID string, msg pb.GameMessage) {

	lock.Lock()
	defer lock.Unlock()
	for singleRoom := range rooms {
		if singleRoom == roomID {
			log.Printf("[Broadcast] Client " + msg.Sender)
			for _, c := range rooms[roomID].roomPlayers {
				log.Printf("[Broadcast] Found " + c + " in room")
				log.Printf("[Broadcast] Adding the message to " + c + "'s channel.")
				if c !=  msg.Sender{
					players[c].channel <- msg
				}
			}
		}
	}
}

func BroadcastGameState(roomID string , msg pb.GameMessage) {
	lock.Lock()
	defer lock.Unlock()
	for singleRoom := range rooms {
		if singleRoom == roomID {
			for _, c := range rooms[roomID].roomPlayers {
				log.Printf("[Broadcast] Found " + c + " in room")
				log.Printf("[Broadcast] Adding the GameState to " + c + "'s channel.")
				players[c].channel <- msg
			}
		}
	}
}

func ClientExists(userID string) bool {

	lock.RLock()
	defer lock.RUnlock()
	for c := range players {
		if c == userID {
			return true
		}
	}

	return false
}

func RemovePlayerFromRoom(playerID string, roomID string) error {

	for _, singleRoom := range rooms {
		if singleRoom.ID.String() == roomID{
			for i, singlePlayer := range singleRoom.roomPlayers {
				if playerID == singlePlayer {
					singleRoom.roomPlayers[i] = singleRoom.roomPlayers[len(singleRoom.roomPlayers) - 1]
					singleRoom.roomPlayers = singleRoom.roomPlayers[:len(singleRoom.roomPlayers) - 1]
				}
			}
			if len(singleRoom.roomPlayers) == 0 {
				delete(rooms, roomID)
			}
		}
	}

	return errors.New("no user found in the group list. Something went wrong")
}

func (s *server) Disconnect(_ context.Context, playerInfo *pb.LeaveRequest) (*pb.Empty, error){

	playerName := playerInfo.PlayerInfo.Name
	roomID := playerInfo.RoomID

	log.Print("[Disconnect]: Unregistering player " + playerName)

	err := RemovePlayer(playerName, roomID)

	if err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func RemovePlayer(playerName string, roomID string) error{

	lock.Lock()
	defer lock.Unlock()

	if ClientExists(playerName) {
		delete(players, playerName)
		log.Print("[RemovePlayer]: Removed Player " + playerName)
		if InRoom(playerName) {
			err := RemovePlayerFromRoom(playerName, roomID)
			if err != nil {
				return errors.New("err while remove player from room")
			}
		} else {
			log.Print("[RemovePlayer]: " + playerName + " was not in any room.")
			return nil
		}
	}

	return errors.New("[RemovePlayer]: Client (" + playerName + ") doesn't exist")
}

func InRoom(playerName string) bool {

	for _, singleRoom := range rooms {
		for _, p := range singleRoom.roomPlayers {
			if playerName == p{
				return true
			}
		}
	}

	return false
}


//func (s *server) LeaveRoom(_ context.Context,leaveRequest *pb.LeaveRequest) (*pb.Empty, error){
//
//	playerID := leaveRequest.PlayerInfo.Uuid
//	roomID := leaveRequest.RoomID
//	msg := pb.GameState{IsPlaying: 3, RoomID: roomID, ScoreTeam1: 0, ScoreTeam2: 0}
//
//	if !RoomExists(roomID) {
//		return &pb.Empty{}, errors.New("the room ID " + roomID + " doesn't exist")
//	} else if !ClientExists(playerID) {
//		return &pb.Empty{}, errors.New("the player " + playerID + " doesn't exist")
//	} else {
//		die := pb.GameMessage{}
//		BroadcastGameState(roomID, die)
//		err := RemovePlayerFromRoom(playerID, roomID)
//		if err != nil {
//			return nil, err
//		}
//		return &pb.Empty{}, nil
//	}
//}