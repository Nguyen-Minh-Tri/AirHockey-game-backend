gen-cal:
	protoc --go_out=plugins=grpc:. pb/airHockey.proto
run-server:
	go run server/server.go server/serverHelper.go server/transferData.go