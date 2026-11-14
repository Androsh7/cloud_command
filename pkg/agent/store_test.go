package agent

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/androsh7/cloud_command/pkg/ec2"
)

func sampleRecord(name string) Record {
	return Record{
		Name:            name,
		Region:          "us-east-1",
		InstanceType:    "t4g.nano",
		Architecture:    ec2.ArchitectureArm64,
		SshUser:         "ec2-user",
		SshPort:         22,
		InstanceId:      "i-123",
		KeyPairName:     "ssh-key-abc",
		AmiId:           "ami-1",
		VpcId:           "vpc-1",
		SecurityGroupId: "sg-1",
		SubnetId:        "subnet-1",
		PublicIp:        "203.0.113.5",
		PrivateKeyFile:  "private_rsa.key",
		PublicKeyFile:   "public_rsa.key",
		CreatedAt:       time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
		Jobs: []Job{{
			Id:         "job1",
			Script:     "sleep 10",
			Sudo:       true,
			StdoutPath: "/tmp/job-job1.stdout",
			StderrPath: "/tmp/job-job1.stderr",
			StartedAt:  time.Date(2026, 10, 5, 12, 5, 0, 0, time.UTC),
		}},
	}
}

func TestStoreSaveAndLoadRoundTripsRecordWithJobs(t *testing.T) {
	store := NewStore(t.TempDir())
	directory, err := store.NewAgentDirectory("alpha")
	if err != nil {
		t.Fatalf("NewAgentDirectory returned error: %v", err)
	}
	expected := sampleRecord("alpha")

	if err := store.Save(&Agent{Record: expected, Directory: directory, store: store}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	loaded, err := store.Load("alpha")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if !reflect.DeepEqual(loaded.Record, expected) {
		t.Fatalf("got %+v, want %+v", loaded.Record, expected)
	}
}

func TestStoreListSortsByName(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		directory, err := store.NewAgentDirectory(name)
		if err != nil {
			t.Fatalf("NewAgentDirectory returned error: %v", err)
		}
		if err := store.Save(&Agent{Record: sampleRecord(name), Directory: directory, store: store}); err != nil {
			t.Fatalf("Save returned error: %v", err)
		}
	}

	agents, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	names := []string{}
	for _, listedAgent := range agents {
		names = append(names, listedAgent.Record.Name)
	}
	if len(names) != 3 || names[0] != "alpha" || names[1] != "bravo" || names[2] != "charlie" {
		t.Fatalf("got %v", names)
	}
}

func TestStoreNewAgentDirectoryRejectsDuplicate(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.NewAgentDirectory("alpha"); err != nil {
		t.Fatalf("first NewAgentDirectory returned error: %v", err)
	}

	_, err := store.NewAgentDirectory("alpha")

	if !errors.Is(err, ErrAgentExists) {
		t.Fatalf("got %v, want ErrAgentExists", err)
	}
}

func TestStoreLoadReportsMissingAgent(t *testing.T) {
	store := NewStore(t.TempDir())

	_, err := store.Load("ghost")

	if !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("got %v, want ErrAgentNotFound", err)
	}
}

func TestValidateNameRejectsPathCharacters(t *testing.T) {
	err := ValidateName("../escape")

	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("got %v, want ErrInvalidName", err)
	}
}
