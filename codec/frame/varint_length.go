/*
 * Copyright 2019 the go-netty project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package frame

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/go-netty/go-netty"
	"github.com/go-netty/go-netty/codec"
	"github.com/go-netty/go-netty/utils"
)

// VarintLengthFieldCodec create varint length field based codec
func VarintLengthFieldCodec(maxFrameLength int) codec.Codec {
	utils.AssertIf(maxFrameLength <= 0, "maxFrameLength must be a positive integer")
	return &varintLengthFieldCodec{
		maxFrameLength: maxFrameLength,
	}
}

type varintLengthFieldCodec struct {
	maxFrameLength int
}

func (v *varintLengthFieldCodec) CodecName() string {
	return "varint-length-field-codec"
}

func (v *varintLengthFieldCodec) HandleRead(ctx netty.InboundContext, message netty.Message) {

	switch r := message.(type) {
	case io.Reader:
		frameLength, err := binary.ReadUvarint(utils.NewByteReader(r))
		utils.Assert(err)
		utils.AssertIf(frameLength > uint64(v.maxFrameLength),
			"frame length too large, frameLength(%d) > maxFrameLength(%d)", frameLength, v.maxFrameLength)

		ctx.HandleRead(io.LimitReader(r, int64(frameLength)))
	case []byte:
		frameLength, n := binary.Uvarint(r)
		utils.AssertIf(frameLength > uint64(v.maxFrameLength),
			"frame length too large, frameLength(%d) > maxFrameLength(%d)", frameLength, v.maxFrameLength)
		utils.AssertIf(int(frameLength) != len(r)-n, "incomplete packet")

		ctx.HandleRead(bytes.NewReader(r[n:]))
	default:
		utils.Assert(fmt.Errorf("unrecognized type: %T", message))
	}
}

func (v *varintLengthFieldCodec) HandleWrite(ctx netty.OutboundContext, message netty.Message) {

	var bodyBytes []byte

	switch r := message.(type) {
	case []byte:
		bodyBytes = r
	case io.Reader:
		bodyBytes = utils.AssertBytes(ioutil.ReadAll(r))
	default:
		utils.Assert(fmt.Errorf("unrecognized type: %T", message))
	}

	utils.AssertIf(len(bodyBytes) > v.maxFrameLength,
		"frame length too large, frameLength(%d) > maxFrameLength(%d)", len(bodyBytes), v.maxFrameLength)

	// encode header
	var head = [binary.MaxVarintLen64]byte{}
	n := binary.PutUvarint(head[:], uint64(len(bodyBytes)))

	// Optimize one merge operation to reduce memory allocation.
	ctx.HandleWrite([][]byte{
		// header
		head[:n],
		// payload
		bodyBytes,
	})

}
