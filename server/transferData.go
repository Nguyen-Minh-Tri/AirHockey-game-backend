package main

import (
	"air-hockey-backend/config/db"
	"air-hockey-backend/model"
	"air-hockey-backend/pb"
	"context"
	"errors"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"log"
)


func RegisterHandler(inName string, inUserName string, inPassword string) error{
	user := &model.User{
		PlayerID: uuid.New().String(),
		Name:     inName,
		UserName: inUserName,
		Password: inPassword,
		Cash: 1000,
	}
	// default cash for new player

	collection, err := db.GetUserCollection()
	if err != nil{
		return errors.New("")
	}
	// find if there is a user which have the same information -> if not then insert
	var result model.User


	userName := bson.D{primitive.E{Key: "userName", Value: inUserName}}
	//user.Name = userName
	err = collection.FindOne(context.TODO(), userName).Decode(&result)

	if err != nil{
		if err.Error() == "mongo: no documents in result"{
			hash, err:= bcrypt.GenerateFromPassword([]byte(inPassword), 5)
			// and then check if hashing password has some problem
			if err != nil{
				return errors.New("hashing err")
			}
			user.Password = string(hash)

			_, err = collection.InsertOne(context.TODO(), &user)
			if err != nil{
				user = nil			// free memory
				return errors.New("create user err")
			}
			return nil
		}
		return err
	}
	user = nil			// free memory
	return errors.New("user name already registered")
}

func LoginHandler(inUserName string, inPassword string) (*model.User, error) {
	// get data from collection users
	collection, err := db.GetUserCollection()

	if err != nil {
		log.Println(err)
		return nil, errors.New("load db user err")
	}
	var result model.User

	err = collection.FindOne(context.TODO(), bson.D{primitive.E{Key: "userName", Value: inUserName}}).Decode(&result)
	if err != nil {
		return nil, errors.New("invalid username")
	}
	//	check password
	err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(inPassword))
	if err != nil {
		return nil, errors.New("invalid password")
	}

	// successfully login
	return &result, nil
}

func storeRecord( inRecord *pb.Record) (string, error){
	recordID := uuid.New()
	record := &model.Record{
		RecordID:   recordID.String(),
		RecordTime: inRecord.GetRecordTime(),
		Team1:      inRecord.GetTeam1(),
		Team2:      inRecord.GetTeam2(),
	}

	collection, err := db.GetRecordCollection()
	if err != nil{
		return "nil", errors.New("[DB] connection err")
	}


	_, err = collection.InsertOne(context.TODO(), record)
	if err != nil{
		return "nil",errors.New("create user err")
	}
	record = nil
	return recordID.String(), nil
}



func updateHighScore( playerRank *model.PlayerRank) error{

	for i, rankRecord := range rankList{
		if playerRank.RankScore > rankList[i].RankScore || &rankList[i] == nil{
			rankRecord.PlayerName 	= 	playerRank.PlayerName
			rankRecord.PlayerID 	= 	playerRank.PlayerID
			rankRecord.RankScore 	= 	playerRank.RankScore
		}
	}

	collection, err := db.GetHighScoreCollection()
	if err != nil{
		return errors.New("[DB] high score table err")
	}

	var resultListRank model.RankingList
	err = collection.FindOne(context.TODO(), bson.D{primitive.E{Key: "rankType", Value: "playerRanking"}}).Decode(&resultListRank)
	if err != nil{
		if err.Error() == "mongo: no documents in result"{
			// insert new record
			newRecord := &model.RankingList{
				RankType: "playerRanking",
				TopRanking: rankList,
			}
			_, err = collection.InsertOne(context.TODO(), &newRecord)
			if err != nil{
				newRecord = nil			// free memory
				return errors.New("new ranking list err")
			}
			return nil
		} else {
			return errors.New("cannot find record")
		}
	}
	// update record if no err
	filter := bson.M{"rankType": "playerRanking"}
	update := bson.M{
		"$set": bson.M{"listHighScore": rankList},
	}

	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}

	updateResult :=  collection.FindOneAndUpdate(context.TODO(), filter, update, &opt)
	if updateResult.Err() != nil {
		return updateResult.Err()
	}

	// decode the result
	doc := bson.M{}
	decodeErr := updateResult.Decode(&doc)
	log.Print(decodeErr)

	return nil
}

func changeRankAndCash(rankAndCast *pb.RankAndCash) error {
	playerID := rankAndCast.PlayerID
	playerCash := rankAndCast.Cash
	playerRank := rankAndCast.Rank

	collection, err := db.GetUserCollection()
	if err != nil{
		return errors.New("[DB] high score table err")
	}

	var resultPlayer model.User
	err = collection.FindOne(context.TODO(), bson.D{primitive.E{Key: "playerID", Value: playerID }}).Decode(&resultPlayer)
	if err != nil{
		if err.Error() == "mongo: no documents in result"{
				return errors.New("no record of user in DB")
		} else {
			return errors.New("cannot find record of user, undetected err")
		}
	}

	// update record if no err
	filter := bson.M{"playerID": playerID}
	update := bson.M{
		"$set": bson.M{"cash": playerCash, "rank": playerRank},
	}

	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}

	updateResult :=  collection.FindOneAndUpdate(context.TODO(), filter, update, &opt)
	if updateResult.Err() != nil {
		return updateResult.Err()
	}

	// decode the result
	doc := bson.M{}
	decodeErr := updateResult.Decode(&doc)
	log.Print(decodeErr)

	return nil
}

func playerUpdateSkin(addSkinReq *pb.AddNewSkin) error{
	playerID := addSkinReq.PlayerID
	skinID := addSkinReq.SkinID
	skinType := addSkinReq.SkinType



	collection, err := db.GetSkinCollection()
	if err != nil{
		return errors.New("[DB] skin table err")
	}

	var resultPlayerSkin model.Skin
	err = collection.FindOne(context.TODO(), bson.D{primitive.E{Key: "playerID", Value: playerID }}).Decode(&resultPlayerSkin)
	if err != nil{
		// case there is no record in the database
		if err.Error() == "mongo: no documents in result"{
			var containerArr []int32
			var emptyArr	 []int32
			switch skinType {
			case 1:
				puck := skinID
				containerArr = append(containerArr, puck)
				newSkin := &model.Skin{
					PlayerID: playerID,
					PuckSkin: containerArr,
					StrikerSkin: emptyArr,
					TableSkin: emptyArr,
				}
				_, err = collection.InsertOne(context.TODO(), &newSkin)
				if err != nil{
					return errors.New("[DB] cannot insert new skin record")
				}
			case 2:
				striker := skinID
				containerArr = append(containerArr, striker)
				newSkin := &model.Skin{
					PlayerID: playerID,
					PuckSkin: emptyArr,
					StrikerSkin: containerArr,
					TableSkin: emptyArr,
				}
				_, err = collection.InsertOne(context.TODO(), &newSkin)
				if err != nil{
					return errors.New("[DB] cannot insert new skin record")
				}
			case 3:
				table := skinID
				containerArr = append(containerArr, table)
				newSkin := &model.Skin{
					PlayerID: playerID,
					PuckSkin: emptyArr,
					StrikerSkin: emptyArr,
					TableSkin: containerArr,
				}
				_, err = collection.InsertOne(context.TODO(), &newSkin)
				if err != nil{
					return errors.New("[DB] cannot insert new skin record")
				}
			}
			return nil
		} else {
			return errors.New("cannot find record of player skin, undetected err")
		}
	}


	// update record if no err - switch case to check which array of skin need to be updated
	filter := bson.M{"playerID": playerID}
	//update := bson.M{
	//	"$set": bson.M{"cash": playerCash, "rank": playerRank},
	//}
	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}

	switch skinType {
	case 1:
		updateVar := append(resultPlayerSkin.PuckSkin, skinID)
		update := bson.M{
			"$set": bson.M{"puckSkin": updateVar},
		}
		updateResult :=  collection.FindOneAndUpdate(context.TODO(), filter, update, &opt)
		if updateResult.Err() != nil {
			return updateResult.Err()
		}
	case 2:
		updateVar := append(resultPlayerSkin.StrikerSkin, skinID)
		update := bson.M{
			"$set": bson.M{"strikerSkin": updateVar},
		}
		updateResult :=  collection.FindOneAndUpdate(context.TODO(), filter, update, &opt)
		if updateResult.Err() != nil {
			return updateResult.Err()
		}
	case 3:
		updateVar := append(resultPlayerSkin.TableSkin, skinID)
		update := bson.M{
			"$set": bson.M{"tableSkin": updateVar},
		}
		updateResult :=  collection.FindOneAndUpdate(context.TODO(), filter, update, &opt)
		if updateResult.Err() != nil {
			return updateResult.Err()
		}

	}

	return nil
}

func  GetSkinByID(playerID int) (*pb.SkinList, error) {
	collection, err := db.GetSkinCollection()
	if err != nil{
		return nil, errors.New("[DB] skin table err")
	}

	var resultPlayerSkin model.Skin
	err = collection.FindOne(context.TODO(), bson.D{primitive.E{Key: "playerID", Value: playerID }}).Decode(&resultPlayerSkin)
	if err != nil{
		return nil, errors.New("[DB] err while load skin")
	}

	returnSkins := &pb.SkinList{
		PuckIDList: resultPlayerSkin.PuckSkin,
		StrikerIDList: resultPlayerSkin.StrikerSkin,
		TableIDList: resultPlayerSkin.TableSkin,
	}

	return returnSkins, nil
}












