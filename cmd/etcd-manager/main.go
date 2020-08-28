/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	apis_etcd "kope.io/etcd-manager/pkg/apis/etcd"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/controller"
	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/locking"
	"kope.io/etcd-manager/pkg/privateapi"
)

func main() {
	flag.Set("logtostderr", "true")

	address := "127.0.0.1"
	flag.StringVar(&address, "address", address, "local address to use")
	peerPort := 2380
	flag.IntVar(&peerPort, "peer-port", peerPort, "peer-port to use")
	clientPort := 4001
	flag.IntVar(&clientPort, "client-port", clientPort, "client-port to use")
	memberCount := 1
	flag.IntVar(&memberCount, "members", memberCount, "initial cluster size; cluster won't start until we have a quorum of this size")
	clusterName := ""
	flag.StringVar(&clusterName, "cluster-name", clusterName, "name of cluster")
	backupStorePath := "/backups"
	flag.StringVar(&backupStorePath, "backup-store", backupStorePath, "backup store location")
	dataDir := "/data"
	flag.StringVar(&dataDir, "data-dir", dataDir, "directory for storing etcd data")
	etcdVersion := "3.2.12"
	flag.StringVar(&etcdVersion, "etcd-version", etcdVersion, "etcd version")

	flag.Parse()

	fmt.Printf("etcd-manager\n")

	if clusterName == "" {
		fmt.Fprintf(os.Stderr, "cluster-name is required\n")
		os.Exit(1)
	}

	if backupStorePath == "" {
		fmt.Fprintf(os.Stderr, "backup-store is required\n")
		os.Exit(1)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		glog.Fatalf("error doing mkdirs on base directory %s: %v", dataDir, err)
	}

	uniqueID, err := privateapi.PersistentPeerId(dataDir)
	if err != nil {
		glog.Fatalf("error getting persistent peer id: %v", err)
	}

	grpcPort := 8000
	discoMe := privateapi.DiscoveryNode{
		ID: uniqueID,
	}
	discoMe.Addresses = append(discoMe.Addresses, privateapi.DiscoveryAddress{
		IP: fmt.Sprintf("%s:%d", address, grpcPort),
	})
	disco, err := privateapi.NewFilesystemDiscovery("/tmp/discovery", discoMe)
	if err != nil {
		glog.Fatalf("error building discovery: %v", err)
	}

	ctx := context.TODO()

	grpcAddress := fmt.Sprintf("%s:%d", address, grpcPort)
	myInfo := privateapi.PeerInfo{
		Id:        string(uniqueID),
		Addresses: []string{address},
	}
	peerServer, err := privateapi.NewServer(ctx, myInfo, disco)
	if err != nil {
		glog.Fatalf("error building server: %v", err)
	}

	var clientUrls []string
	clientUrls = append(clientUrls, fmt.Sprintf("http://%s:%d", address, clientPort))

	var peerUrls []string
	peerUrls = append(peerUrls, fmt.Sprintf("http://%s:%d", address, peerPort))

	etcdNodeInfo := &apis_etcd.EtcdNode{
		Name:       string(uniqueID),
		ClientUrls: clientUrls,
		PeerUrls:   peerUrls,
	}

	backupStore, err := backup.NewStore(backupStorePath)
	if err != nil {
		glog.Fatalf("error initializing backup store: %v", err)
	}

	etcdServer := etcd.NewEtcdServer(dataDir, clusterName, etcdNodeInfo, peerServer)
	go etcdServer.Run(ctx)

	spec := &protoetcd.ClusterSpec{
		MemberCount: int32(memberCount),
		EtcdVersion: etcdVersion,
	}
	initialClusterState := controller.StaticInitialClusterSpecProvider(spec)

	var leaderLock locking.Lock // nil
	c, err := controller.NewEtcdController(leaderLock, backupStore, clusterName, peerServer, initialClusterState)
	if err != nil {
		glog.Fatalf("error building etcd controller: %v", err)
	}
	go c.Run(ctx)

	if err := peerServer.ListenAndServe(ctx, grpcAddress); err != nil {
		if ctx.Err() == nil {
			glog.Fatalf("error creating private API server: %v", err)
		}
	}

	os.Exit(0)
}
