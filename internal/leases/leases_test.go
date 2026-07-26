package leases

import "testing"

func TestHashStableAndTokenRandom(t *testing.T) {
	tokenA, hashA, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	tokenB, hashB, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if tokenA == tokenB {
		t.Fatal("expected random lease tokens")
	}
	if Hash(tokenA) != hashA {
		t.Fatal("hash mismatch")
	}
	if hashA == hashB {
		t.Fatal("expected different token hashes")
	}
}
