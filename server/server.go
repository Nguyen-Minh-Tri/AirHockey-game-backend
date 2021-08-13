package main

import (
	"air-hockey-backend/model"
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
//important global variable
var rankList [10]model.PlayerRank
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
//  ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~@!!!!!!!!@!@!@!@!!!!!!~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~


func (s *server) NewAccount(_ context.Context, accountReq *pb.NewAccountReq) (*pb.Empty, error){
	name := accountReq.Name
	info := accountReq.AccountInfo
	errorAccount := RegisterHandler(name, info.UserName, info.Password)
	if errorAccount != nil{
		return nil, errorAccount
	}
	return &pb.Empty{}, nil
}

func (s *server) Login(_ context.Context, account *pb.Account) (*pb.LoginPlayerInfo, error) {

	result, errLogin :=LoginHandler(account.UserName, account.Password)
	if errLogin != nil{
		return nil, errLogin
	}
	AddPlayer(result.PlayerID,account.UserName)
	log.Print("[AddedPlayer], ", result.Name, result.PlayerID, result.Cash, result.Rank )
	return &pb.LoginPlayerInfo{Name: result.Name, Uuid: result.PlayerID, Cash: int32(result.Cash), Rank: int32(result.Rank)}, nil
}

func (s *server) NewRecord(_ context.Context, inRecord *pb.Record) (*pb.RecordID, error) {

	uuid, errNewRec := storeRecord(inRecord)
	if errNewRec != nil{
		return nil, errNewRec
	}
	// insert record
	returnID := &pb.RecordID{Uuid: uuid}
	return returnID, nil
}

func (s *server) GetGlobalRecord(context.Context, *pb.Empty) (*pb.RankingList, error) {
	if &rankList == nil{
		return nil, errors.New("no record found")
	}
	var result pb.RankingList
	for i, _ := range rankList{
		result.RankingList = append(result.RankingList, &pb.PlayerRank {PlayerName: rankList[i].PlayerName, RankScore : int32(rankList[i].RankScore)})
	}
	return &result, nil
}

func (s *server) UpdateRankAndCash(_ context.Context, rankAndCast *pb.RankAndCash) (*pb.Empty, error){
	err := changeRankAndCash(rankAndCast)
	if err == nil{
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *server) AddSkin(_ context.Context, addSkinReq *pb.AddNewSkin) (*pb.SkinList, error){
	err := playerUpdateSkin(addSkinReq)
	if err != nil{
		return nil, err
	}

	return nil, nil
}

func (s *server) GetSkinList(_ context.Context, playerID *pb.PlayerID) (*pb.SkinList, error){
	playerSkins, err := GetSkinByID(int(playerID.GetID()))
	if err != nil {
		return nil, err
	}
	return playerSkins, nil
}

//  ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~@!!!!!!!!@!@!@!@!!!!!!~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

//func (s *server) Connect(_ context.Context, in *pb.NewPlayerName) (*pb.PlayerInfo, error) {
//	name := in.Name												// each user need an uuid for their connection, later merge to their account
//	id := uuid.New()
//	AddPlayer(id, name)
//	return &pb.PlayerInfo{Name: name, Uuid: id.String()}, nil
//}

func (s *server) NewRoom(_ context.Context, in *pb.NewGameInfo) (*pb.RoomID, error) {
	lock.Lock()													// each room will be initialized with a locker, max score of game, then add to the global list of all room
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
	roomID := in.RoomID																				// take the roomID and join, if empty, then server will find it
	if roomID == ""{																				// case empty: find an empty room in the room list
		for _, singleRoom := range rooms{
			if len(singleRoom.roomPlayers) != int(singleRoom.maxPlayer) {
				singleRoom.roomPlayers = append(singleRoom.roomPlayers, in.PlayerInfo.Uuid)
				roomID = singleRoom.ID.String()
			}
			break
		}
		return &pb.RoomID{UniqueID: roomID}, nil
	}

	if RoomExists(in.RoomID ) {																		// case found
		AddPlayerToRoom(in.PlayerInfo.Uuid, in.RoomID)
		return &pb.RoomID{UniqueID: in.RoomID}, nil
	}

	return nil, errors.New("room does not exist")												// case all room are full
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
	outbox := make(chan pb.GameMessage, 100)
	go ListenToClient(svr, outbox)

	for {
		select {
		//case <-svr.Context().Done():											// 1. GET interruption from client
		//	RemoveInterruptPlayer()												// https://stackoverflow.com/questions/39825671/grpc-go-how-to-know-in-server-side-when-client-closes-the-connection
		//	return nil
		case outMsg := <-outbox:												// 2. HERE message from outbox
			log.Print("Recv some message")

			switch outMsg.GetAction().(type) {
			case *pb.GameMessage_GameState :										// 2a. broadcast game state for: start game, end game, left room
				BroadcastGameState(outMsg.GetGameState().RoomID, outMsg)
				log.Print("[GAME_STATE]")
			case *pb.GameMessage_EntityState:										// 2b. broadcast entity position to player who is not host player
				BroadcastToPlayer(outMsg.GetEntityState().RoomID, outMsg)
				log.Print("[ENTITY]")
			case *pb.GameMessage_PlayerInput:										// 2c. broadcast game input from player to host
				log.Print("[INPUT]")
				hostID := GetHost(outMsg)
				BroadcastToHost(hostID, outMsg)
			case *pb.GameMessage_Empty:												// 2d. open connection to server need a empty message
				log.Println("[Init] Hand shake message")
			case nil:
				log.Print("[NIL_ACTION] end of client")
			}
		case inMsg := <-players[req.Sender].channel:							// 3. SEND message to suitable channel
			log.Print(players[req.Sender].channel)
			err := svr.Send(&inMsg)
			if err != nil {
				return err
			}
		}
	}
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

func (s *server) LeaveRoom(_ context.Context,leaveRequest *pb.LeaveRequest) (*pb.Empty, error){

	playerID := leaveRequest.PlayerInfo.Uuid
	roomID := leaveRequest.RoomID

	if !RoomExists(roomID) {
		return &pb.Empty{}, errors.New("the room ID " + roomID + " doesn't exist")
	} else if !ClientExists(playerID) {
		return &pb.Empty{}, errors.New("the player ID " + playerID + " doesn't exist")
	} else {
		err := RemovePlayerFromRoom(playerID, roomID)
		if err != nil {
			return nil, err
		}
		return &pb.Empty{}, nil
	}
}


