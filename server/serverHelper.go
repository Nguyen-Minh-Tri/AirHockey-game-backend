package main

import (
	"air-hockey-backend/pb"
	"errors"
	"io"
	"log"
	"sync"
)

func AddPlayer(id string, name string) {
	lock.Lock()
	defer lock.Unlock()
	newPlayer := &Player{
		name:      			name,
		channel: 		 	make(chan pb.GameMessage, 100),
		WaitGroup:			&sync.WaitGroup{},
		uuid: 				id,
	}
	log.Print("[AddClient]: Registered player: " + name)
	players[id] = newPlayer
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

func GetHost(playerInput pb.GameMessage) string{
	roomID := playerInput.GetPlayerInput().RoomID
	for _, singleRoom := range rooms{
		if singleRoom.ID.String() == roomID{
			return singleRoom.host
		}
	}
	return "Wrong room ID"
}

func BroadcastToSpecificClient(clientID string, msg pb.GameMessage){
	lock.Lock()
	defer lock.Unlock()
	players[clientID].channel <- msg
}

func ListenToClient(svr pb.AirHockeyService_GameStreamServer, messages chan<- pb.GameMessage) {
	for {
		req, err := svr.Recv()
		if err == io.EOF {
			log.Printf("[ListenToClient] A player had been disconnected")
			return
		}
		if err != nil{
			log.Print(err)
			return
		}else {
			messages <- *req
		}
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
					if len(singleRoom.roomPlayers) == 0 {
						delete(rooms, roomID)
					}
					return nil
				}
			}

		}
	}

	return errors.New("no user found in the group list. Something went wrong")
}

func RemovePlayer(playerName string, roomID string) error{


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

func RemoveInterruptPlayer(playerID string) error{

	lock.Lock()
	defer lock.Unlock()

	if ClientExists(playerID) {
		delete(players, playerID)
		log.Print("[RemovePlayer]: Removed Player " + playerID)
		err := RemovePlayerFromContext(playerID)
		if err != nil {
			return errors.New("err while remove player from room")
		}
		return nil
	}

	return errors.New("[RemovePlayer]: Client (" + playerID + ") doesn't exist")
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

func RemovePlayerFromContext(playerID string) error {

	for _, singleRoom := range rooms {
		for _, p := range singleRoom.roomPlayers {
			if playerID == p{
				err := RemovePlayerFromRoom(playerID, singleRoom.ID.String())
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}