/*
 *  Copyright (C) 2020-2021  AnySwap Ltd. All rights reserved.
 *  Copyright (C) 2020-2021  haijun.cai@anyswap.exchange
 *
 *  This library is free software; you can redistribute it and/or
 *  modify it under the Apache License, Version 2.0.
 *
 *  This library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
 *
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package keygen

import (
	"errors"
	//"fmt"
	"github.com/anyswap/FastMulThreshold-DSA/smpc/tss/smpc"
	"github.com/anyswap/FastMulThreshold-DSA/tss-lib/ec2"
	"github.com/anyswap/FastMulThreshold-DSA/smpc/socket"
	"github.com/anyswap/FastMulThreshold-DSA/log"
	"encoding/json"
)

// Start broacast commitment D 
func (round *round3) Start() error {
	if round.started {
		return errors.New("round already started")
	}
	round.number = 3
	round.started = true
	round.ResetOK()

	curIndex, err := round.GetDNodeIDIndex(round.dnodeid)
	if err != nil {
		return err
	}

	ids, err := round.GetIDs()
	if err != nil {
		return err
	}

	if round.tee {
	    return round.ExecTee(curIndex)
	}
	
	// add for GG20: keygen phase 3. Each player Pi proves in ZK that Ni is square-free using the proof of Gennaro, Micciancio, and Rabin [30]
	// An Efficient Non-Interactive Statistical Zero-Knowledge Proof System for Quasi-Safe Prime Products, section 3.1
	for k := range ids {
	    msg1, ok := round.temp.kgRound1Messages[k].(*KGRound1Message)
	    if !ok {
		return errors.New("round.Start get round1 msg fail")
	    }

	    paiPk := msg1.U1PaillierPk
	    if paiPk == nil {
		    return errors.New("error kg round1 message")
	    }

	    msg22, ok := round.temp.kgRound2Messages2[k].(*KGRound2Message2)
	    if !ok {
		return errors.New("round.Start get round2 msg 2 fail")
	    }

	    if !ec2.SquareFreeVerify(paiPk.N,msg22.Num,msg22.SfPf) {
		log.Error("keygen round3,check that a zero-knowledge proof that paillier.N is a square-free integer fail","k",ids[k])
		return errors.New("check that a zero-knowledge proof that paillier.N is a square-free integer fail")
	    }
	}

	kg := &KGRound3Message{
		KGRoundMessage: new(KGRoundMessage),
		ComU1GD:        round.temp.commitU1G.D,
		ComC1GD:        round.temp.commitC1G.D,
		U1PolyGG:       round.temp.u1PolyG.PolyG,
	}
	kg.SetFromID(round.dnodeid)
	kg.SetFromIndex(curIndex)
	round.temp.kgRound3Messages[curIndex] = kg
	round.out <- kg

	//fmt.Printf("========= round3 start success, u1polygg = %v, k = %v ==========\n", round.temp.u1PolyG.PolyG, curIndex)
	return nil
}

// CanAccept is it legal to receive this message 
func (round *round3) CanAccept(msg smpc.Message) bool {
	if _, ok := msg.(*KGRound3Message); ok {
		return msg.IsBroadcast()
	}
	/*if _, ok := msg.(*KGRound3Message1); ok {
		return !msg.IsBroadcast()
	}*/
	return false
}

// Update  is the message received and ready for the next round? 
func (round *round3) Update() (bool, error) {
	for j, msg := range round.temp.kgRound3Messages {
		if round.ok[j] {
			continue
		}
		if msg == nil || !round.CanAccept(msg) {
			return false, nil
		}
		/*msg31 := round.temp.kgRound3Messages1[j]
		if msg31 == nil || !round.CanAccept(msg31) {
			return false, nil
		}*/
		round.ok[j] = true
	}
	return true, nil
}

// NextRound enter next round
func (round *round3) NextRound() smpc.Round {
	round.started = false
	return &round4{round}
}

//---------------------------------

func (round *round3) ExecTee(curIndex int) error {
    ids, err := round.GetIDs()
    if err != nil {
	return err
    }

    var vdata string

    for k := range ids {
	msg1, ok := round.temp.kgRound1Messages[k].(*KGRound1Message)
	if !ok {
	    return errors.New("round.Start get round1 msg fail")
	}

	paiPk := msg1.U1PaillierPk
	if paiPk == nil {
	    return errors.New("error kg round1 message")
	}

	msg22, ok := round.temp.kgRound2Messages2[k].(*KGRound2Message2)
	if !ok {
	    return errors.New("round.Start get round2 msg 2 fail")
	}

	s := &socket.KGRound3Msg{N:paiPk.N, Num:msg22.Num, SfPf: msg22.SfPf}
	s.Base.SetBase(round.keytype,round.msgprex)
	err := socket.SendMsgData(smpc.VSocketConnect,s)
	if err != nil {
	    return err
	}
       
	kgs := <-round.teeout
	msgmap := make(map[string]string)
	err = json.Unmarshal([]byte(kgs), &msgmap)
	if err != nil {
	    log.Error("===============round3 start,unmarshal KGRound3 error","err",err)
	    return err
	}

	if msgmap["SquareFreeVerifyRes"] == "FALSE" {
	    log.Error("keygen round3,check that a zero-knowledge proof that paillier.N is a square-free integer fail","k",ids[k])
	    return errors.New("check that a zero-knowledge proof that paillier.N is a square-free integer fail")
	}

	vdata = msgmap["TeeValidateData"]
    }

    kg := &KGRound3Message{
	    KGRoundMessage: new(KGRoundMessage),
	    ComU1GD:        round.temp.commitU1G.D,
	    ComC1GD:        round.temp.commitC1G.D,
	    U1PolyGG:       round.temp.u1PolyG.PolyG,
    }
    kg.SetFromID(round.dnodeid)
    kg.SetFromIndex(curIndex)
    kg.SetTeeValidateData(vdata)
    round.temp.kgRound3Messages[curIndex] = kg
    round.out <- kg

    return nil
}

