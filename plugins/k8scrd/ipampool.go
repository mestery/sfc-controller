// Copyright (c) 2018 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8scrd

import (
	"os"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller"
	crd "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/apis/sfccontroller/v1alpha1"
	model "github.com/ligato/sfc-controller/plugins/controller/model"
)

type CRDIpamPoolMgr struct {
	IpamPoolCache map[string]string
}

func (mgr *CRDIpamPoolMgr) Init() {
}

func (mgr *CRDIpamPoolMgr) AfterInit() {
	mgr.InitRAMCache()
	go mgr.InitAndRunWatcher()
}

// InitRAMCache create a map for all the entities
func (mgr *CRDIpamPoolMgr) InitRAMCache() {
	//mgr.IpamPoolCache = nil // delete old cache for re-init
	//mgr.IpamPoolCache = make(map[string]*IpamPool)
}

// DumpCache logs all the entries in the map
func (mgr *CRDIpamPoolMgr) DumpCache() {
	for _, nn := range mgr.IpamPoolCache {
		log.Printf("IpamPool: %s", nn)
	}
}

func (mgr *CRDIpamPoolMgr) CrdToSfcIpamPool(crdIP crd.IpamPool) (sfcIP controller.IPAMPool, err error) {
	sfcIP = controller.IPAMPool{}
	sfcIP.Metadata = &model.MetaDataType{}
	sfcIP.Metadata.Name = crdIP.Name
	sfcIP.Metadata.Labels = crdIP.Labels
	sfcIP.IPAMPool.Spec = &crdIP.IPAMPoolSpec
	sfcIP.IPAMPool.Status = &crdIP.IPAMPoolStatus
	return sfcIP, nil
}

// HandleCRDSync syncs the SFC Controller with the incoming CRD
func (mgr *CRDIpamPoolMgr) HandleCRDSync(crdIP crd.IpamPool) {
	log.Info("CRDIpamPoolMgr HandleCRDSync: enter ...")
	defer log.Info("CRDIpamPoolMgr HandleCRDSync: exit ...")

	ip, err := mgr.CrdToSfcIpamPool(crdIP)
	if err != nil {
		log.Errorf("%s", err.Error())
		return
	}

	opStr := "created"
	if existing, exists := ctlrPlugin.IpamPoolMgr.HandleCRUDOperationR(crdIP.Name); exists {
		opStr = "updated"
		if existing.ConfigEqual(&ip) {
			log.Infof("crdIP %s has not changed.", crdIP.Name)
			return
		}
	}

	if err := ctlrPlugin.IpamPoolMgr.HandleCRUDOperationCU(&ip, false); err != nil {
		log.Errorf("%s", err.Error())
		return
	}

	log.Infof("IpamPool %s", opStr)
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *CRDIpamPoolMgr) InitAndRunWatcher() {

	log.Info("CRD IpamPoolWatcher: enter ...")
	defer log.Info("CRD IpamPoolWatcher: exit ...")

	respChan := make(chan keyval.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(ctlrPlugin.IpamPoolMgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("CRD IpamPoolWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("CRD IpamPoolWatcher: watching the key: %s", ctlrPlugin.IpamPoolMgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:

			switch resp.GetChangeType() {
			case datasync.Put:
				dbEntry := &controller.IPAMPool{}
				if err := resp.GetValue(dbEntry); err == nil {
					// config and status might have changed ...
					log.Infof("CRD IpamPoolWatcher: PUT detected: IpamPool: %s",
						dbEntry)
				}

			case datasync.Delete:
				log.Infof("CRD IpamPoolWatcher: deleting key: %s ", resp.GetKey())
				// tell k8s crd that resource has been removed
			}
		}
	}
}