package aws

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type mockKMSClient struct {
	rotationStatusCalls []string
	mu                  sync.Mutex
	keys                []kmstypes.KeyListEntry
	describe            func(context.Context, *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error)
	errOperation        string
}

func (m *mockKMSClient) ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	if m.errOperation == "list keys" {
		return nil, errors.New("boom")
	}
	if m.keys != nil {
		return &kms.ListKeysOutput{Keys: m.keys}, nil
	}
	return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: awssdk.String("c")}, {KeyId: awssdk.String("b")}, {KeyId: awssdk.String("a")}}}, nil
}
func (m *mockKMSClient) DescribeKey(ctx context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	if m.errOperation == "describe key" {
		return nil, errors.New("boom")
	}
	if m.describe != nil {
		return m.describe(ctx, in)
	}
	id := awssdk.ToString(in.KeyId)
	manager := kmstypes.KeyManagerTypeCustomer
	keySpec := kmstypes.KeySpecSymmetricDefault
	keyState := kmstypes.KeyStateEnabled
	if id == "b" {
		keySpec = kmstypes.KeySpecRsa2048
	} else if id == "c" {
		keyState = kmstypes.KeyStateDisabled
	}
	return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{KeyId: in.KeyId, Arn: awssdk.String("arn:" + id), KeyState: keyState, KeyManager: manager, KeySpec: keySpec, Origin: kmstypes.OriginTypeAwsKms}}, nil
}
func (m *mockKMSClient) ListAliases(context.Context, *kms.ListAliasesInput, ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	if m.errOperation == "list aliases" {
		return nil, errors.New("boom")
	}
	return &kms.ListAliasesOutput{Aliases: []kmstypes.AliasListEntry{{AliasName: awssdk.String("alias/app"), TargetKeyId: awssdk.String("a")}}}, nil
}
func (m *mockKMSClient) GetKeyRotationStatus(_ context.Context, in *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	if m.errOperation == "rotation status" {
		return nil, errors.New("boom")
	}
	id := awssdk.ToString(in.KeyId)
	m.mu.Lock()
	m.rotationStatusCalls = append(m.rotationStatusCalls, id)
	m.mu.Unlock()
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: id == "a"}, nil
}

func TestListKMSKeysWrapsAPIErrors(t *testing.T) {
	for _, test := range []struct{ operation, want string }{
		{"list aliases", "list KMS aliases"},
		{"list keys", "list KMS keys"},
		{"describe key", "describe KMS key"},
		{"rotation status", "rotation status for KMS key"},
	} {
		t.Run(test.operation, func(t *testing.T) {
			_, err := (&AwsRepository{KMSClient: &mockKMSClient{errOperation: test.operation}}).ListKMSKeys(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q context, got %v", test.want, err)
			}
		})
	}
}

func TestListKMSKeysMapsAliasesRotationAndSorts(t *testing.T) {
	client := &mockKMSClient{}
	repo := &AwsRepository{KMSClient: client}
	keys, err := repo.ListKMSKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 || keys[0].ID != "a" || len(keys[0].Aliases) != 1 || keys[0].Aliases[0] != "alias/app" || !keys[0].RotationEligible || !keys[0].RotationEnabled {
		t.Fatalf("unexpected keys: %+v", keys)
	}
	if keys[1].Manager != "CUSTOMER" || keys[1].RotationEligible || keys[1].RotationEnabled {
		t.Fatalf("unexpected asymmetric customer key: %+v", keys[1])
	}
	if keys[2].State != "Disabled" || !keys[2].RotationEligible || keys[2].RotationEnabled {
		t.Fatalf("unexpected disabled customer key: %+v", keys[2])
	}
	sort.Strings(client.rotationStatusCalls)
	if !slices.Equal(client.rotationStatusCalls, []string{"a", "c"}) {
		t.Fatalf("expected rotation status for supported keys, got calls for %v", client.rotationStatusCalls)
	}
}

func TestListKMSKeysBoundsConcurrentDetailLoads(t *testing.T) {
	var active, peak atomic.Int32
	release := make(chan struct{})
	var releaseOnce sync.Once
	client := &mockKMSClient{}
	client.describe = func(ctx context.Context, in *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := peak.Load()
			if current <= previous || peak.CompareAndSwap(previous, current) {
				break
			}
		}
		if current == 10 {
			releaseOnce.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{KeyId: in.KeyId}}, nil
	}
	client.keys = make([]kmstypes.KeyListEntry, 20)
	for i := range client.keys {
		client.keys[i].KeyId = awssdk.String(string(rune('a' + i)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := (&AwsRepository{KMSClient: client}).ListKMSKeys(ctx); err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 10 {
		t.Fatalf("expected concurrency capped at 10, got %d", peak.Load())
	}
}
