package protocol

type Guard func(command Command) Reply
