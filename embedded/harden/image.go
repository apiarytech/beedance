/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package harden

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
	"time"

	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/vm"
)

// Image is a program as a PLC runs it: its initialization, its body, and
// what the PLC needs to know of its variables.
type Image struct {
	Init, Scan *compiler.Bytecode
	// Globals is the number of global slots the program uses.
	Globals int
	// RetainSlots are the global slots of the RETAIN variables, and
	// RetainLayout their fingerprint (Layout).
	RetainSlots  []int
	RetainLayout uint32
}

// Checksum is the image's checksum: Checksum(im.Init, im.Scan).
func (im Image) Checksum() uint32 { return Checksum(im.Init, im.Scan) }

// ImageOf returns the image of a compiled PROGRAM
// (compiler.CompileProgramUnit).
func ImageOf(prog *compiler.CompiledProgram) Image {
	im := Image{
		Init:    prog.InitBytecode,
		Scan:    prog.CyclicBytecode,
		Globals: max(vm.GlobalsNeeded(prog.InitBytecode), vm.GlobalsNeeded(prog.CyclicBytecode)),
	}
	var layout []string
	for _, v := range prog.Variables {
		if v.Retain && !v.Constant && v.Global >= 0 {
			im.RetainSlots = append(im.RetainSlots, v.Global)
			layout = append(layout, v.Name, v.Type)
		}
	}
	im.RetainLayout = Layout(layout...)
	return im
}

// Errors of DecodeImage.
var (
	// ErrImageFormat is the error of data that is not a whole, sound image:
	// spoilt in transit or storage, cut short, or not an image at all.
	ErrImageFormat = errors.New("harden: not a sound program image")
	// ErrIncompatible is the error of an image that calls a built-in
	// function this firmware does not have at the same index: built by
	// another version of beedance.
	ErrIncompatible = errors.New("harden: program image built for other firmware")
)

// An encoded image is: magic, payload length, payload, then the CRC-32 of
// all that, little-endian.
var imageMagic = [4]byte{'B', 'D', 'I', '1'}

// EncodeImage returns im as bytes for DecodeImage: what is sent to a PLC
// to change its program (PLC.BeginUpdate). It fails on a constant it
// cannot encode. object.FinalizeBuiltins must have run.
func EncodeImage(im Image) ([]byte, error) {
	var p []byte
	p = appendU32(p, uint32(im.Globals))
	p = appendU32(p, im.RetainLayout)
	p = appendU32(p, uint32(len(im.RetainSlots)))
	for _, s := range im.RetainSlots {
		p = appendU32(p, uint32(s))
	}
	refs, err := builtinRefs(im.Init, im.Scan)
	if err != nil {
		return nil, err
	}
	p = appendU32(p, uint32(len(refs)))
	for _, i := range refs {
		if i >= len(object.Builtins) {
			return nil, errors.New("harden: program calls an unknown built-in")
		}
		p = appendString(appendU32(p, uint32(i)), object.Builtins[i].Name)
	}
	for _, bc := range []*compiler.Bytecode{im.Init, im.Scan} {
		p = appendString(p, string(bc.Instructions))
		p = appendU32(p, uint32(len(bc.Constants)))
		for _, c := range bc.Constants {
			if fn, ok := c.(*object.CompiledFunction); ok {
				p = append(appendFunction(p, fn), fn.Instructions...)
				continue
			}
			var ok bool
			if p, ok = appendValue(p, c); !ok {
				return nil, errors.New("harden: a constant of type " + string(c.Type()) + " cannot be encoded")
			}
		}
	}
	out := append(imageMagic[:], appendU32(nil, uint32(len(p)))...)
	out = append(out, p...)
	return appendU32(out, crc32.ChecksumIEEE(out)), nil
}

// imageLen returns the length of the encoded image data starts with, or
// 0 when data does not start with a sound one.
func imageLen(data []byte) int {
	if len(data) < 12 || !bytes.Equal(data[:4], imageMagic[:]) {
		return 0
	}
	n := int(binary.LittleEndian.Uint32(data[4:]))
	if n < 0 || n > len(data)-12 {
		return 0
	}
	end := 8 + n
	if crc32.ChecksumIEEE(data[:end]) != binary.LittleEndian.Uint32(data[end:]) {
		return 0
	}
	return end + 4
}

// DecodeImage returns the image EncodeImage made data from, as new
// objects. It checks data's CRC, and that the built-in functions the
// program calls are this firmware's.
func DecodeImage(data []byte) (Image, error) {
	n := imageLen(data)
	if n == 0 {
		return Image{}, ErrImageFormat
	}
	d := &decoder{b: data[8 : n-4]}
	im := Image{Globals: int(d.u32()), RetainLayout: d.u32()}
	for k := d.u32(); k > 0 && !d.err; k-- {
		im.RetainSlots = append(im.RetainSlots, int(d.u32()))
	}
	for k := d.u32(); k > 0 && !d.err; k-- {
		i, name := int(d.u32()), d.str()
		if !d.err && (i >= len(object.Builtins) || object.Builtins[i].Name != name) {
			return Image{}, ErrIncompatible
		}
	}
	for _, bc := range []**compiler.Bytecode{&im.Init, &im.Scan} {
		*bc = &compiler.Bytecode{Instructions: []byte(d.str())}
		for k := d.u32(); k > 0 && !d.err; k-- {
			(*bc).Constants = append((*bc).Constants, d.any())
		}
	}
	if d.err || len(d.b) != 0 || im.Globals <= 0 {
		return Image{}, ErrImageFormat
	}
	for _, s := range im.RetainSlots {
		if s >= im.Globals {
			return Image{}, ErrImageFormat
		}
	}
	return im, nil
}

// any reads a constant: a value appendValue wrote, but a struct, or a
// function appendFunction wrote with its instructions after.
func (d *decoder) any() object.Object {
	switch tag := d.u8(); tag {
	case tagNull:
		return vm.Null
	case tagBool:
		if d.u8() != 0 {
			return vm.True
		}
		return vm.False
	case tagSInt:
		return &object.SInt{Value: int8(d.u64())}
	case tagInt:
		return &object.Int{Value: int16(d.u64())}
	case tagDInt:
		return &object.DInt{Value: int32(d.u64())}
	case tagLInt:
		return &object.LInt{Value: int64(d.u64())}
	case tagUSInt:
		return &object.USInt{Value: uint8(d.u64())}
	case tagUInt:
		return &object.UInt{Value: uint16(d.u64())}
	case tagUDInt:
		return &object.UDInt{Value: uint32(d.u64())}
	case tagULInt:
		return &object.ULInt{Value: d.u64()}
	case tagReal:
		return &object.Real{Value: math.Float64frombits(d.u64())}
	case tagLReal:
		return &object.LReal{Value: math.Float64frombits(d.u64())}
	case tagByte:
		return &object.Byte{Value: byte(d.u64())}
	case tagWord:
		return &object.Word{Value: uint16(d.u64())}
	case tagDWord:
		return &object.DWord{Value: uint32(d.u64())}
	case tagLWord:
		return &object.LWord{Value: d.u64()}
	case tagBitString:
		w := int(d.u8())
		return &object.BitString{Width: w, Value: d.u64()}
	case tagString:
		return &object.String{Value: d.str()}
	case tagWString:
		return &object.WString{Value: d.str()}
	case tagTime:
		return &object.Time{Value: time.Duration(d.u64())}
	case tagDate:
		y, m, day := int(int32(d.u32())), time.Month(d.u8()), int(d.u8())
		return &object.Date{Value: time.Date(y, m, day, 0, 0, 0, 0, time.UTC)}
	case tagTimeOfDay:
		return &object.TimeOfDay{Value: time.Unix(0, int64(d.u64())).UTC()}
	case tagDateAndTime:
		return &object.DateAndTime{Value: time.Unix(0, int64(d.u64())).UTC()}
	case tagArray:
		a := &object.Array{LowerBound: int64(d.u64())}
		n := int(d.u32())
		if n > len(d.b) {
			d.err = true
			return vm.Null
		}
		for ; n > 0 && !d.err; n-- {
			a.Elements = append(a.Elements, d.any())
		}
		return a
	case tagFunction:
		fn := &object.CompiledFunction{NumLocals: int(d.u32()), NumParameters: int(d.u32())}
		for _, names := range []*[]string{&fn.ParameterNames, &fn.OutputNames} {
			for k := d.u32(); k > 0 && !d.err; k-- {
				*names = append(*names, d.str())
			}
		}
		for k := d.u32(); k > 0 && !d.err; k-- {
			fn.OutputIndices = append(fn.OutputIndices, int(d.u64()))
		}
		n := int(d.u32())
		if n > len(d.b) {
			d.err = true
			return vm.Null
		}
		fn.Instructions = append([]byte(nil), d.next(n)...)
		return fn
	}
	d.err = true
	return vm.Null
}

// builtinRefs returns the built-in function indexes bcs's instructions
// read with OpGetBuiltin, those of compiled functions among the constants
// too.
func builtinRefs(bcs ...*compiler.Bytecode) ([]int, error) {
	seen := map[int]bool{}
	var refs []int
	scan := func(ins []byte) error {
		for ip := 0; ip < len(ins); {
			def, err := code.Lookup(ins[ip])
			if err != nil {
				return err
			}
			if code.Opcode(ins[ip]) == code.OpGetBuiltin && ip+2 < len(ins) {
				if i := int(code.ReadUint16(ins[ip+1:])); !seen[i] {
					seen[i] = true
					refs = append(refs, i)
				}
			}
			ip++
			for _, w := range def.OperandWidths {
				ip += w
			}
		}
		return nil
	}
	for _, bc := range bcs {
		if err := scan(bc.Instructions); err != nil {
			return nil, err
		}
		for _, c := range bc.Constants {
			if fn, ok := c.(*object.CompiledFunction); ok {
				if err := scan(fn.Instructions); err != nil {
					return nil, err
				}
			}
		}
	}
	return refs, nil
}
