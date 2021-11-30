package main

import (
	"fmt"
	"log"
	"time"

	pts "github.com/sftwrngnr/sampleproject/pkg"
)

func TestClientConnect(ipaddy string) {
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	req.ReqCmd = pts.GetStatus
	for {
		retstat = new(pts.ReturnStatus)
		pts.RpcClientRequest(&req, ipaddy, retstat)
		if retstat.CurState == pts.EmptyStatus {
			t1 := time.NewTimer(5 * time.Second)
			<-t1.C
			continue
		}
		log.Printf("Status: %v\n", *retstat)
		if !retstat.MoreDataAvail && (retstat.CurState == pts.TestComplete || retstat.CurState == pts.RetrievingDataFilesComplete) {
			log.Printf("Done retrieving %d", retstat.CurState)
			break
		}
	}
}

func main() {
	fmt.Println("TestRPCClient")
	TestClientConnect("127.0.0.1")
}
