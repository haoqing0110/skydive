/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package packet_injector

import (
	"fmt"
	"net/http"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	// Namespace Packet_Injector
	Namespace = "Packet_Injector"
)

// PacketInjectorServer creates a packet injector server API
type PacketInjectorServer struct {
	Graph    *graph.Graph
	Channels *channels
}

func (pis *PacketInjectorServer) stopPI(msg *shttp.WSStructMessage) error {
	var uuid string
	if err := msg.DecodeObj(&uuid); err != nil {
		return err
	}
	pis.Channels.Lock()
	c, ok := pis.Channels.Pipes[uuid]
	pis.Channels.Unlock()
	if ok {
		c <- true
		return nil
	}
	return fmt.Errorf("No PI running on this ID: %s", uuid)
}

func (pis *PacketInjectorServer) injectPacket(msg *shttp.WSStructMessage) (string, error) {
	var params PacketInjectionParams
	if err := msg.DecodeObj(&params); err != nil {
		return "", fmt.Errorf("Unable to decode packet inject param message %v", msg)
	}

	trackingID, err := InjectPackets(&params, pis.Graph, pis.Channels)
	if err != nil {
		return "", fmt.Errorf("Failed to inject packet: %s", err.Error())
	}

	return trackingID, nil
}

// OnWSMessage event, websocket PIRequest message
func (pis *PacketInjectorServer) OnWSStructMessage(c shttp.WSSpeaker, msg *shttp.WSStructMessage) {
	switch msg.Type {
	case "PIRequest":
		var reply *shttp.WSStructMessage
		trackingID, err := pis.injectPacket(msg)
		replyObj := &PacketInjectorReply{TrackingID: trackingID}
		if err != nil {
			logging.GetLogger().Error(err)

			replyObj.Error = err.Error()
			reply = msg.Reply(replyObj, "PIResult", http.StatusBadRequest)
		} else {
			reply = msg.Reply(replyObj, "PIResult", http.StatusOK)
		}

		c.SendMessage(reply)
	case "PIStopRequest":
		var reply *shttp.WSStructMessage
		err := pis.stopPI(msg)
		replyObj := &PacketInjectorReply{}
		if err != nil {
			replyObj.Error = err.Error()
			reply = msg.Reply(replyObj, "PIStopResult", http.StatusBadRequest)
		} else {
			reply = msg.Reply(replyObj, "PIStopResult", http.StatusOK)
		}
		c.SendMessage(reply)
	}
}

// NewServer creates a new packet injector server API based on websocket server
func NewServer(graph *graph.Graph, pool shttp.WSStructSpeakerPool) *PacketInjectorServer {
	s := &PacketInjectorServer{
		Graph:    graph,
		Channels: &channels{Pipes: make(map[string](chan bool))},
	}
	pool.AddStructMessageHandler(s, []string{Namespace})
	return s
}
