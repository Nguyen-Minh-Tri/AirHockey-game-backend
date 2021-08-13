package model

import (
	_ "sync"
)

type User struct {
	PlayerID   string   `bson:"playerID"`
	Name       string   `bson:"name"`
	UserName   string   `bson:"userName"`
	Password   string   `bson:"password"`
	Cash       int      `bson:"cash"`
	Rank 	   int 		`bson:"rank"`
	RecordList []string `bson:"recordList"`				// list of record uuid
}

type Record struct {
	RecordID   string   `bson:"recordID"`
	RecordTime string   `bson:"recordTime"`
	Team1      []string `bson:"team1"` 					// list userName of Team1
	Team2      []string `bson:"team2"` 					// list userName of Team2
	MatchScore [2]int	`bson:"matchScore"`
}

type RankingList struct {
	RankType	string			`bson:"rankType"`		// can be ranking score, cash or win streak
	TopRanking	[10]PlayerRank	`bson:"listHighScore"`	// 10 uuid of record
}

type Skin struct {
	PlayerID   	string   	`bson:"playerID"`
	PuckSkin	[]int32		`bson:"puckSkin"`
	StrikerSkin	[]int32		`bson:"strikerSkin"`
	TableSkin	[]int32		`bson:"tableSkin"`
}

// PlayerRank -- supported model
type PlayerRank struct {
	PlayerID	string
	PlayerName	string
	RankScore 	int
}

