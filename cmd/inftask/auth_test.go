package main

import "testing"

func TestCloudAuthRequiresConfiguration(t *testing.T) {
	for _, key := range []string{"INFONE_UC_BASE_URL", "INFONE_UC_CLIENT_ID", "INFONE_UC_CLIENT_SECRET", "INFTASK_PUBLIC_URL", "INFONE_UC_JWT_ISSUER", "INFONE_SHARE_DIR"} {
		t.Setenv(key, "")
	}
	for _, addr := range []string{"127.0.0.1:8787", "[::1]:8787", "localhost:8787"} {
		if a, err := cloudAuth(addr); err != nil || a != nil {
			t.Fatal(addr, err)
		}
	}
	for _, addr := range []string{":8787", "0.0.0.0:8787", "192.168.1.2:8787", "bad"} {
		if _, err := cloudAuth(addr); err == nil {
			t.Fatal("public unauthenticated listener accepted", addr)
		}
	}
	t.Setenv("INFONE_UC_CLIENT_ID", "harnesskit")
	if _, err := cloudAuth("127.0.0.1:8787"); err == nil {
		t.Fatal("partial config fell back to unauthenticated mode")
	}
}
