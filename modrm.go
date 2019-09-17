package main

type ModRM struct {
	Mod uint8
	//opecodeとreg_indexと共用
	Reg uint8
	Rm  uint8

	Sib    uint8
	Disp8  int8
	Disp32 uint32
}
