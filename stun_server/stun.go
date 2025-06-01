package stun_server

import (
	"fmt"
	"github.com/pion/stun"
	"net"
)

func GetPacket(numBytes int, clientAddr *net.UDPAddr, buffer []byte) ([]byte, error) {
	msg := &stun.Message{
		Raw: make([]byte, numBytes),
	}

	copy(msg.Raw, buffer[:numBytes]) // Make a copy, as buf will be reused

	fmt.Println(msg)
	var err = msg.Decode()
	if err != nil {
		fmt.Println("Error decoding STUN message:", err)
		return nil, fmt.Errorf("Error occured in decoding the message", err)
	}

	fmt.Println(msg.Type)
	if msg.Type == stun.MessageType(stun.BindingRequest) {

		response, err := stun.Build(
			stun.BindingSuccess,
			stun.NewTransactionIDSetter(msg.TransactionID),
			&stun.XORMappedAddress{
				IP:   clientAddr.IP,
				Port: clientAddr.Port,
			})

		if err != nil {
			fmt.Println("this is the error", err)
			return nil, fmt.Errorf("Error building STUN response", err)
		}

		fmt.Println("sending the packet")
		return response.Raw, nil

	} else {
		fmt.Println("Recieved non Binding", msg.Type)
		return nil, fmt.Errorf("Received non-BindingRequest STUN message typ", msg.Type)
	}
}
