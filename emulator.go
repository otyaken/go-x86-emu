package main

import (
	"fmt"
	"io"
	"os"
)

// レジスタ番号
const (
	CEax = iota
	CEcx
	CEdx
	CEbx
	CEsp
	CEbp
	CEsi
	CEdi
	RegistersCount
)

const (
	CAl = CEax
	CAh = CEax + 4
	CCl = CEcx
	CCh = CEcx + 4
	CDl = CEdx
	CDh = CEdx + 4
	CBl = CEbx
	CBh = CEbx + 4
)

// eflags関連
const (
	CarryFlag    = uint32(1)
	ZeroFlag     = uint32(1 << 6)
	SignFlag     = uint32(1 << 7)
	OverFlowFlag = uint32(1 << 11)
)

type Emulator struct {
	// 汎用レジスタ
	Registers [RegistersCount]uint32
	// eflagsレジスタ
	Eflags uint32
	// メモリ
	Memory []uint8
	// EIPレジスタ
	Eip uint32
	// 最大メモリ
	MaxMemorySize uint32
}

func (e *Emulator) DumpEmulator() {
	fmt.Printf("EAX = 0x%08x\n", e.Registers[CEax])
	fmt.Printf("ECX = 0x%08x\n", e.Registers[CEcx])
	fmt.Printf("EDX = 0x%08x\n", e.Registers[CEdx])
	fmt.Printf("EBX = 0x%08x\n", e.Registers[CEbx])
	fmt.Printf("ESP = 0x%08x\n", e.Registers[CEsp])
	fmt.Printf("EBP = 0x%08x\n", e.Registers[CEbp])
	fmt.Printf("ESI = 0x%08x\n", e.Registers[CEsi])
	fmt.Printf("EDI = 0x%08x\n", e.Registers[CEdi])
	fmt.Printf("EIP = 0x%08x\n", e.Eip)
}

func (e *Emulator) ParseModrm() *ModRM {
	modRM := &ModRM{}
	code := e.GetCode8(0)
	//  76 543 210
	//  01 101 111
	// Mod Reg RM
	modRM.Mod = uint8((code & 0xc0) >> 6)
	modRM.Reg = uint8((code & 0x38) >> 3)
	modRM.Rm = uint8(code & 0x07)

	e.Eip += 1

	// SIBの存在確認。
	// 存在すれば、後続のSIBを取得。
	if modRM.Mod != 3 && modRM.Rm == 4 {
		modRM.Sib = uint8(e.GetCode8(0))
		e.Eip += 1
	}
	if (modRM.Mod == 0 && modRM.Rm == 5) || modRM.Mod == 2 {
		modRM.Disp32 = uint32(e.GetSignCode32(0))
		e.Eip += 4
	} else if modRM.Mod == 1 {
		modRM.Disp8 = int8(e.GetSignCode8(0))
		e.Eip += 1
	}
	return modRM
}

func (e *Emulator) GetCode8(index uint32) uint32 {
	return uint32(e.Memory[e.Eip+index])
}

func (e *Emulator) GetSignCode8(index uint32) int32 {
	val := e.Memory[e.Eip+index]
	sign := val >> 7
	// 符号が正の時、そのままint32に変換しリターン。
	if sign == 0 {
		return int32(val)
	}
	// 負の数をuint8の値から計算。
	// 符号が負の時、2の補数をとりint32に変換することにより、負の値の大きさを取得。
	// その後マイナスを掛け、リターン。
	return -(int32((^val + 1)))
}

func (e *Emulator) GetSignCode32(index uint32) int32 {

	val := e.GetCode32(index)

	sign := val >> 31
	// 符号が正の時、そのままint32に変換しリターン。
	if sign == 0 {
		return int32(val)
	}
	// 負の数をuint8の値から計算。
	// 符号が負の時、2の補数をとりint32に変換することにより、負の値の大きさを取得。
	// その後マイナスを掛け、リターン。
	return -(int32((^val + 1)))
}

func (e *Emulator) GetCode32(index uint32) uint32 {
	var (
		ret uint32
		i   uint32
	)

	for i = 0; i < 4; i++ {
		ret |= e.GetCode8(index+i) << (i * 8)
	}
	return ret
}

func (e *Emulator) MovR32Imm32() {
	reg := e.GetCode8(0) - 0xB8
	value := e.GetCode32(1)
	e.Registers[reg] = uint32(value)
	e.Eip += 5
}

func (e *Emulator) ShortJump() {
	diff := e.GetSignCode8(1)
	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) NearJump() {
	diff := e.GetSignCode32(1)
	if diff > 0 {
		e.Eip += uint32(diff) + 5
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 5
	}
}

func (e *Emulator) MovRm32Imm32() {
	e.Eip += 1
	modRM := e.ParseModrm()
	value := e.GetCode32(0)
	e.Eip += 4
	e.setRm32(modRM, value)
}

func (e *Emulator) setRegister32(index uint8, value uint32) {
	e.Registers[index] = value
}

func (e *Emulator) getRegister32(index uint8) uint32 {
	return e.Registers[index]
}

// SIBは実装していない。
func (e *Emulator) calcMemoryAddress(m *ModRM) uint32 {
	if m.Mod == 0 {
		if m.Rm == 4 {
			fmt.Printf("Not implemented ModRM mod = 0, rm = 4\n")
			os.Exit(0)
		} else if m.Rm == 5 {
			return m.Disp32
		} else {
			return e.getRegister32(m.Rm)
		}
	} else if m.Mod == 1 {
		if m.Rm == 4 {
			fmt.Printf("Not implemented ModRM mod = 1, rm = 4\n")
			os.Exit(0)
		} else {
			if m.Disp8 > 0 {
				return e.getRegister32(m.Rm) + uint32(m.Disp8)
			} else {
				return e.getRegister32(m.Rm) - uint32(-m.Disp8)
			}
		}
	} else if m.Mod == 2 {
		if m.Rm == 4 {
			fmt.Printf("Not implemented ModRM mod = 0, rm = 4\n")
			os.Exit(0)
		} else {
			return e.getRegister32(m.Rm) + m.Disp32
		}
	} else {
		fmt.Printf("Not implemented ModRM rm = 3\n")
		os.Exit(0)
	}
	// エラー。設計について再検討する必要あり。
	return 0
}

func (e *Emulator) setMemory8(address uint32, value uint32) {
	e.Memory[address] = uint8(value & 0xFF)
}

func (e *Emulator) getMemory8(address uint32) uint8 {
	return e.Memory[address]
}

func (e *Emulator) setMemory32(address uint32, value uint32) {
	var i uint32
	for i = 0; i < 4; i++ {
		e.setMemory8(address+i, (value>>(i*8))&0xFF)
	}
}

func (e *Emulator) getMemory32(address uint32) uint32 {
	var (
		i   uint32
		ret uint32
	)

	for i = 0; i < 4; i++ {
		ret |= uint32(e.getMemory8(address+i)) << (i * 8)
	}
	return ret
}

func (e *Emulator) MovRm32R32() {
	e.Eip += 1
	modRM := e.ParseModrm()
	r32 := e.getR32(modRM)
	e.setRm32(modRM, r32)
}

func (e *Emulator) MovR32Rm32() {
	e.Eip += 1
	modRM := e.ParseModrm()
	rm32 := e.getRm32(modRM)
	e.setR32(modRM, rm32)
}

func (e *Emulator) AddRm32R32() {
	e.Eip += 1
	modRM := e.ParseModrm()
	r32 := e.getR32(modRM)
	rm32 := e.getRm32(modRM)
	e.setRm32(modRM, r32+rm32)
}

func (e *Emulator) AddRm32Imm8(m *ModRM) {
	rm32 := e.getRm32(m)
	imm8 := e.GetSignCode8(0)
	e.Eip += 1
	e.setRm32(m, rm32+uint32(imm8))
}

func (e *Emulator) Code83() {
	e.Eip += 1
	modRM := e.ParseModrm()
	//modRM.Regはopecodeも表す。
	switch modRM.Reg {
	case 0:
		e.AddRm32Imm8(modRM)
	case 5:
		e.SubRm32Imm8(modRM)
	case 7:
		e.CmpRm32Imm8(modRM)
	default:
		fmt.Printf("Not implemented: 83 %d\n", modRM.Reg)
		os.Exit(1)
	}
}

func (e *Emulator) IncRm32(m *ModRM) {
	value := e.getRm32(m)
	e.setRm32(m, value+1)
}

func (e *Emulator) CodeFF() {
	e.Eip += 1
	modRM := e.ParseModrm()
	//modRM.Regはopecodeも表す。
	switch modRM.Reg {
	case 0:
		e.IncRm32(modRM)
	default:
		fmt.Printf("Not implemented: FF %d\n", modRM.Reg)
		os.Exit(1)
	}
}

func (e *Emulator) PushR32() {
	reg := e.GetCode8(0) - 0x50
	e.push32(e.getRegister32(uint8(reg)))
	e.Eip += 1
}

func (e *Emulator) push32(value uint32) {
	address := e.getRegister32(CEsp) - 0x4
	e.setRegister32(CEsp, address)
	e.setMemory32(address, value)
}

func (e *Emulator) getR32(m *ModRM) uint32 {
	return e.getRegister32(m.Reg)
}

func (e *Emulator) popR32() {
	reg := e.GetCode8(0) - 0x58
	e.setRegister32(uint8(reg), e.pop32())
	e.Eip += 1
}

func (e *Emulator) pop32() uint32 {
	address := e.getRegister32(CEsp)
	ret := e.getMemory32(address)
	e.setRegister32(CEsp, address+0x4)
	return ret
}

func (e *Emulator) CallRel32() {
	diff := e.GetSignCode32(1)
	e.push32(e.Eip + 5)
	if diff > 0 {
		e.Eip += uint32(diff) + 5
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 5
	}
}

func (e *Emulator) Ret() {
	e.Eip = e.pop32()
}

func (e *Emulator) Leave() {
	ebp := e.getRegister32(CEbp)
	e.setRegister32(CEsp, ebp)
	e.setRegister32(CEbp, e.pop32())
	e.Eip += 1
}

func (e *Emulator) PushImm32() {
	value := e.GetCode32(1)
	e.push32(value)
	e.Eip += 5
}

func (e *Emulator) PushImm8() {
	value := e.GetCode8(1)
	e.push32(value)
	e.Eip += 2
}

func (e *Emulator) setR32(m *ModRM, value uint32) {
	e.setRegister32(m.Reg, value)
}

func (e *Emulator) getRm32(m *ModRM) uint32 {
	if m.Mod == 3 {
		return e.getRegister32(m.Rm)
	}
	address := e.calcMemoryAddress(m)
	return e.getMemory32(address)
}

func (e *Emulator) setRm32(m *ModRM, value uint32) {
	if m.Mod == 3 {
		e.setRegister32(m.Rm, value)
	} else {
		address := e.calcMemoryAddress(m)
		e.setMemory32(address, value)
	}
}

func (e *Emulator) CmpR32Rm32() {
	e.Eip += 1
	modRM := e.ParseModrm()
	r32 := e.getR32(modRM)
	rm32 := e.getRm32(modRM)
	result := uint64(r32) - uint64(rm32)
	e.updateEflagsSub(r32, rm32, result)
}

func (e *Emulator) CmpRm32Imm8(m *ModRM) {
	rm32 := e.getRm32(m)
	imm8 := uint32(e.GetSignCode8(0))
	e.Eip += 1
	result := uint64(rm32) - uint64(imm8)
	e.updateEflagsSub(rm32, imm8, result)
}

func (e *Emulator) SubRm32Imm8(m *ModRM) {
	rm32 := e.getRm32(m)
	imm8 := e.GetSignCode8(0)
	e.Eip += 1
	result := uint64(rm32) - uint64(imm8)
	e.setRm32(m, uint32(result))
	e.updateEflagsSub(rm32, uint32(imm8), result)
}

func (e *Emulator) updateEflagsSub(v1 uint32, v2 uint32, result uint64) {
	sign1 := v1 >> 31
	sign2 := v2 >> 31
	signr := (result >> 31) & 0x01

	e.setCarry((result >> 32) != 0)
	e.setZero(result == 0)
	e.setSign(signr == 1)
	e.setOverFlow(sign1 != sign2 && sign1 != uint32(signr))
}

func (e *Emulator) setCarry(b bool) {
	if b {
		e.Eflags |= CarryFlag
	} else {
		e.Eflags &= ^CarryFlag
	}
}

func (e *Emulator) setZero(b bool) {
	if b {
		e.Eflags |= ZeroFlag
	} else {
		e.Eflags &= ^ZeroFlag
	}
}

func (e *Emulator) setSign(b bool) {
	if b {
		e.Eflags |= SignFlag
	} else {
		e.Eflags &= ^SignFlag
	}
}

func (e *Emulator) setOverFlow(b bool) {
	if b {
		e.Eflags |= OverFlowFlag
	} else {
		e.Eflags &= ^OverFlowFlag
	}
}

func (e *Emulator) isCarry() bool {
	return e.Eflags&CarryFlag != 0
}

func (e *Emulator) isZero() bool {
	return e.Eflags&ZeroFlag != 0
}

func (e *Emulator) isSign() bool {
	return e.Eflags&SignFlag != 0
}

func (e *Emulator) isOverFlow() bool {
	return e.Eflags&OverFlowFlag != 0
}

func (e *Emulator) Js() {
	var diff int32
	if e.isSign() {
		diff = e.GetSignCode8(1)
	} else {
		diff = 0
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jns() {
	var diff int32
	if e.isSign() {
		diff = 0
	} else {
		diff = e.GetSignCode8(1)
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jc() {
	var diff int32
	if e.isCarry() {
		diff = e.GetSignCode8(1)
	} else {
		diff = 0
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jnc() {
	var diff int32
	if e.isCarry() {
		diff = 0
	} else {
		diff = e.GetSignCode8(1)
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jz() {
	var diff int32
	if e.isZero() {
		diff = e.GetSignCode8(1)
	} else {
		diff = 0
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jnz() {
	var diff int32
	if e.isZero() {
		diff = 0
	} else {
		diff = e.GetSignCode8(1)
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jo() {
	var diff int32
	if e.isOverFlow() {
		diff = e.GetSignCode8(1)
	} else {
		diff = 0
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jno() {
	var diff int32
	if e.isOverFlow() {
		diff = 0
	} else {
		diff = e.GetSignCode8(1)
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jl() {
	var diff int32
	if e.isSign() != e.isOverFlow() {
		diff = e.GetSignCode8(1)
	} else {
		diff = 0
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) Jle() {
	var diff int32
	if e.isZero() || e.isSign() != e.isOverFlow() {
		diff = e.GetSignCode8(1)
	} else {
		diff = 0
	}

	if diff > 0 {
		e.Eip += uint32(diff) + 2
	} else {
		e.Eip -= uint32(-diff)
		e.Eip += 2
	}
}

func (e *Emulator) InAlDx() {
	address := e.getRegister32(CEdx) & 0xFFFF
	value := e.IoIn8(uint16(address))
	e.setRegister8(CAl, value)
	e.Eip += 1
}

func (e *Emulator) OutDxAl() {
	address := e.getRegister32(CEdx) & 0xFFFF
	value := e.getRegister8(CAl)
	e.IoOut8(uint16(address), value)
	e.Eip += 1
}

func (e *Emulator) IoIn8(address uint16) uint8 {
	switch address {
	case 0x03F8:
		var a uint32
		fmt.Scan(&a)
		return uint8(a)
	default:
		return 0
	}
}

func (e *Emulator) IoOut8(address uint16, value uint8) {
	switch address {
	case 0x03F8:
		fmt.Printf("%c\n", value)
	}
}

func (e *Emulator) getRegister8(index uint8) uint8 {
	if index < 4 {
		return uint8(e.Registers[index] & 0xFF)
	} else {
		//Highを返すので8ビット右シフト
		return uint8((e.Registers[index-4] >> 8) & 0xFF)
	}
}

func (e *Emulator) setRegister8(index uint8, value uint8) {
	if index < 4 {
		v := e.Registers[index] & 0xFFFFFF00
		e.Registers[index] = v | uint32(value)
	} else {
		v := e.Registers[index-4] & 0xFFFF00FF
		e.Registers[index-4] = v | (uint32(value) << 8)
	}
}

func (e *Emulator) MovR8Imm8() {
	e.Eip += 1
	modRM := e.ParseModrm()
	rm8 := e.getRm8(modRM)
	e.setR8(modRM, rm8)
}

func (e *Emulator) getRm8(m *ModRM) uint8 {
	if m.Mod == 3 {
		return e.getRegister8(m.Rm)
	} else {
		address := e.calcMemoryAddress(m)
		return e.getMemory8(address)
	}
}

func (e *Emulator) setRm8(m *ModRM, value uint8) {
	if m.Mod == 3 {
		e.setRegister8(m.Rm, value)
	} else {
		address := e.calcMemoryAddress(m)
		e.setMemory8(address, uint32(value))
	}
}

func (e *Emulator) setR8(m *ModRM, value uint8) {
	e.setRegister8(m.Reg, value)
}

func (e *Emulator) getR8(m *ModRM) uint8 {
	return e.getRegister8(m.Reg)
}

func (e *Emulator) MovRm8R8() {
	e.Eip += 1
	modRM := e.ParseModrm()
	r8 := e.getR8(modRM)
	e.setRm8(modRM, r8)

}

func (e *Emulator) MovR8Rm8() {
	e.Eip += 1
	modRM := e.ParseModrm()
	rm8 := e.getRm8(modRM)
	e.setR8(modRM, rm8)

}

func (e *Emulator) CmpAlImm8() {
	value := e.GetCode8(1)
	al := e.getRegister8(CAl)
	var result uint64
	result = uint64(al) - uint64(value)
	e.updateEflagsSub(uint32(al), value, result)
	e.Eip += 2
}

func (e *Emulator) CmpEaxImm32() {
	value := e.GetCode32(1)
	eax := e.getRegister32(CEax)
	var result uint64
	result = uint64(eax) - uint64(value)
	e.updateEflagsSub(eax, value, result)
	e.Eip += 5
}

func (e *Emulator) IncR32() {
	reg := e.GetCode8(0) - 0x40
	e.setRegister32(uint8(reg), e.getRegister32(uint8(reg))+1)
	e.Eip += 1
}

// ソフトウェア割込み
func (e *Emulator) Swi() {
	intIndex := e.GetCode8(1)
	e.Eip += 2

	switch intIndex {
	case 0x10:
		e.biosVideo()
	default:
		fmt.Printf("Unknown interrupt: 0x%02x\n", intIndex)
	}
}

func (e *Emulator) biosVideo() {
	funcIndex := e.getRegister8(CAh)
	switch funcIndex {
	case 0x0e:
		e.biosVideoTeletype()
	default:
		fmt.Printf("Not implemented BIOS video function: 0x%02x\n", funcIndex)
	}
}

// alレジスタに格納された文字コードをblレジスタに格納された文字色で画面に描画
func (e *Emulator) biosVideoTeletype() {
	biosToTerminal := [...]int{30, 34, 32, 36, 31, 35, 33, 37}
	color := e.getRegister8(CBl) & 0x0F
	char := e.getRegister8(CAl)
	terminalColor := biosToTerminal[color&0x07]
	bright := (color & 0x08) >> 3
	str := fmt.Sprintf("\x1b[%d;%dm%c\x1b[0m]", bright, terminalColor, char)
	e.putString(str)
}

func (e *Emulator) putString(s string) {
	for i := 0; i < len(s); i++ {
		e.IoOut8(0x03F8, s[i])
	}
}

func (e *Emulator) executeOpCode(opCode uint32) {
	switch opCode {
	case 0x01:
		e.AddRm32R32()

	case 0x3B:
		e.CmpR32Rm32()
	case 0x3C:
		e.CmpAlImm8()
	case 0x3D:
		e.CmpEaxImm32()

	case 0x40, 0x40 + 1, 0x40 + 2, 0x40 + 3, 0x40 + 4, 0x40 + 5, 0x40 + 6, 0x40 + 7:
		e.IncR32()

	case 0x50, 0x50 + 1, 0x50 + 2, 0x50 + 3, 0x50 + 4, 0x50 + 5, 0x50 + 6, 0x50 + 7:
		e.PushR32()

	case 0x58, 0x58 + 1, 0x58 + 2, 0x58 + 3, 0x58 + 4, 0x58 + 5, 0x58 + 6, 0x58 + 7:
		e.popR32()

	case 0x68:
		e.PushImm32()
	case 0x6A:
		e.PushImm8()
	case 0x70:
		e.Jo()
	case 0x71:
		e.Jno()
	case 0x72:
		e.Jc()
	case 0x73:
		e.Jnc()
	case 0x74:
		e.Jz()
	case 0x75:
		e.Jnz()
	case 0x78:
		e.Js()
	case 0x79:
		e.Jns()
	case 0x7C:
		e.Jl()
	case 0x7E:
		e.Jle()

	case 0x83:
		e.Code83()
	case 0x88:
		e.MovRm8R8()
	case 0x89:
		e.MovRm32R32()
	case 0x8A:
		e.MovRm32R32()
	case 0x8B:
		e.MovR32Rm32()

	case 0xB0, 0xB0 + 1, 0xB0 + 2, 0xB0 + 3, 0xB0 + 4, 0xB0 + 5, 0xB0 + 6, 0xB0 + 7:
		e.MovR8Imm8()

	case 0xB8, 0xB8 + 1, 0xB8 + 2, 0xB8 + 3, 0xB8 + 4, 0xB8 + 5, 0xB8 + 6, 0xB8 + 7:
		e.MovR32Imm32()

	case 0xC3:
		e.Ret()
	case 0xC7:
		e.MovRm32Imm32()
	case 0xC9:
		e.Leave()

	case 0xCD:
		e.Swi()

	case 0xE8:
		e.CallRel32()
	case 0xE9:
		e.NearJump()
	case 0xEB:
		e.ShortJump()
	case 0xEE:
		e.OutDxAl()
	case 0xFF:
		e.CodeFF()
		// 実装されていない命令に対応する処理。
		// 実装されていない命令を読み込んだら、VMを終了させる。
	default:
		fmt.Printf("\nNot Implemented: %x", opCode)
		e.Eip = 0
	}
}

func (e *Emulator) Run(quiet bool) {
	for {
		if e.Eip >= e.MaxMemorySize {
			break
		}
		code := e.GetCode8(0)
		if !quiet {
			fmt.Printf("EIP = %x, Code = %02x\n", e.Eip, code)
		}
		e.executeOpCode(code)
		if e.Eip == 0x0 {
			fmt.Printf("\nend of program.\n\n")
			break
		}
	}
}

func NewEmulator(memorySize uint32, eip uint32, esp uint32, fileName string) (*Emulator, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, memorySize)
	count := 0
	for {
		c, err := f.Read(buf)
		if err == io.EOF {
			count += c
			break
		}
		if err != nil {
			return nil, err
		}
		count += c
	}

	// []byteから[]uint8へ、スライスの型変換。
	memory := make([]uint8, len(buf))
	for i := 0; i < count; i++ {
		memory[i+0x7c00] = uint8(buf[i])
	}

	emu := &Emulator{
		Memory:        memory,
		Eip:           eip,
		MaxMemorySize: memorySize,
	}
	emu.Registers[CEsp] = esp

	return emu, nil
}
