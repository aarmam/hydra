package hsm_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"reflect"
	"testing"

	"github.com/ThalesIgnite/crypto11"
	"github.com/golang/mock/gomock"
	"github.com/miekg/pkcs11"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/cryptosigner"

	"github.com/ory/hydra/hsm"
	"github.com/ory/hydra/x"
)

func TestKeyManager_GenerateKeySet(t *testing.T) {
	ctrl := gomock.NewController(t)
	hsmContext := NewMockContext(ctrl)
	defer ctrl.Finish()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 512)
	require.NoError(t, err)

	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	require.NoError(t, err)

	rsaKeyPair := NewMockSignerDecrypter(ctrl)
	rsaKeyPair.EXPECT().Public().Return(&rsaKey.PublicKey).AnyTimes()

	ecdsaKeyPair := NewMockSignerDecrypter(ctrl)
	ecdsaKeyPair.EXPECT().Public().Return(&ecdsaKey.PublicKey).AnyTimes()

	var kid = uuid.New()

	type args struct {
		ctx context.Context
		set string
		kid string
		alg string
		use string
	}
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		args    args
		want    *jose.JSONWebKeySet
		wantErr bool
	}{
		{
			name: "Generate RS256",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
				alg: "RS256",
				use: "sig",
			},
			setup: func(t *testing.T) {
				privateAttrSet, publicAttrSet := expectedKeyAttributes(t, kid)
				hsmContext.EXPECT().GenerateRSAKeyPairWithAttributes(gomock.Eq(publicAttrSet), gomock.Eq(privateAttrSet), gomock.Eq(4096)).Return(rsaKeyPair, nil)
			},
			want: expectedKeySet(rsaKeyPair, kid, "RS256", "sig"),
		},
		{
			name: "Generate ES256",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
				alg: "ES256",
				use: "sig",
			},
			setup: func(t *testing.T) {
				privateAttrSet, publicAttrSet := expectedKeyAttributes(t, kid)
				hsmContext.EXPECT().GenerateECDSAKeyPairWithAttributes(gomock.Eq(publicAttrSet), gomock.Eq(privateAttrSet), gomock.Eq(elliptic.P256())).Return(ecdsaKeyPair, nil)
			},
			want: expectedKeySet(ecdsaKeyPair, kid, "ES256", "sig"),
		},
		{
			name: "Generate ES512",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
				alg: "ES512",
				use: "sig",
			},
			setup: func(t *testing.T) {
				privateAttrSet, publicAttrSet := expectedKeyAttributes(t, kid)
				hsmContext.EXPECT().GenerateECDSAKeyPairWithAttributes(gomock.Eq(publicAttrSet), gomock.Eq(privateAttrSet), gomock.Eq(elliptic.P521())).Return(ecdsaKeyPair, nil)
			},
			want: expectedKeySet(ecdsaKeyPair, kid, "ES512", "sig"),
		},
		{
			name: "Generate unsupported",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
				alg: "ES384",
				use: "sig",
			},
			setup:   func(t *testing.T) {},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			m := &hsm.KeyManager{
				Context: hsmContext,
			}
			got, err := m.GenerateKeySet(tt.args.ctx, tt.args.set, tt.args.kid, tt.args.alg, tt.args.use)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateKeySet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateKeySet() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyManager_GetKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	hsmContext := NewMockContext(ctrl)
	defer ctrl.Finish()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 512)
	require.NoError(t, err)
	rsaKeyPair := NewMockSignerDecrypter(ctrl)
	rsaKeyPair.EXPECT().Public().Return(&rsaKey.PublicKey).AnyTimes()

	ecdsaP256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecdsaP256KeyPair := NewMockSignerDecrypter(ctrl)
	ecdsaP256KeyPair.EXPECT().Public().Return(&ecdsaP256Key.PublicKey).AnyTimes()

	ecdsaP521Key, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	require.NoError(t, err)
	ecdsaP521KeyPair := NewMockSignerDecrypter(ctrl)
	ecdsaP521KeyPair.EXPECT().Public().Return(&ecdsaP521Key.PublicKey).AnyTimes()

	var kid = uuid.New()

	type args struct {
		ctx context.Context
		set string
		kid string
	}
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		args    args
		want    *jose.JSONWebKeySet
		wantErr bool
	}{
		{
			name: "Get RS256 sig",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(rsaKeyPair, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(rsaKeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(nil, nil)
			},
			want: expectedKeySet(rsaKeyPair, kid, "RS256", "sig"),
		},
		{
			name: "Get RS256 enc",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(rsaKeyPair, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(rsaKeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(pkcs11.NewAttribute(pkcs11.CKA_ENCRYPT, true), nil)
			},
			want: expectedKeySet(rsaKeyPair, kid, "RS256", "enc"),
		},
		{
			name: "Key usage attribute error",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(rsaKeyPair, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(rsaKeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(nil, errors.New(""))
			},
			want: expectedKeySet(rsaKeyPair, kid, "RS256", "sig"),
		},
		{
			name: "Get ES256 sig",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(ecdsaP256KeyPair, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(ecdsaP256KeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(nil, nil)
			},
			want: expectedKeySet(ecdsaP256KeyPair, kid, "ES256", "sig"),
		},
		{
			name: "Get ES256 enc",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(ecdsaP256KeyPair, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(ecdsaP256KeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(pkcs11.NewAttribute(pkcs11.CKA_ENCRYPT, true), nil)
			},
			want: expectedKeySet(ecdsaP256KeyPair, kid, "ES256", "enc"),
		},
		{
			name: "Get ES512 sig",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(ecdsaP521KeyPair, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(ecdsaP521KeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(nil, nil)
			},
			want: expectedKeySet(ecdsaP521KeyPair, kid, "ES512", "sig"),
		},
		{
			name: "Get ES512 enc",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(ecdsaP521KeyPair, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(ecdsaP521KeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(pkcs11.NewAttribute(pkcs11.CKA_ENCRYPT, true), nil)
			},
			want: expectedKeySet(ecdsaP521KeyPair, kid, "ES512", "enc"),
		},
		{
			name: "Key not found",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(nil, nil)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			m := &hsm.KeyManager{
				Context: hsmContext,
			}
			got, err := m.GetKey(tt.args.ctx, tt.args.set, tt.args.kid)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetKey() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyManager_GetKeySet(t *testing.T) {
	ctrl := gomock.NewController(t)
	hsmContext := NewMockContext(ctrl)
	defer ctrl.Finish()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 512)
	require.NoError(t, err)
	rsaKid := uuid.New()
	rsaKeyPair := NewMockSignerDecrypter(ctrl)
	rsaKeyPair.EXPECT().Public().Return(&rsaKey.PublicKey).AnyTimes()

	ecdsaP256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecdsaP256Kid := uuid.New()
	ecdsaP256KeyPair := NewMockSignerDecrypter(ctrl)
	ecdsaP256KeyPair.EXPECT().Public().Return(&ecdsaP256Key.PublicKey).AnyTimes()

	ecdsaP521Key, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)

	require.NoError(t, err)
	ecdsaP521Kid := uuid.New()
	ecdsaP521KeyPair := NewMockSignerDecrypter(ctrl)
	ecdsaP521KeyPair.EXPECT().Public().Return(&ecdsaP521Key.PublicKey).AnyTimes()

	allKeys := []crypto11.Signer{rsaKeyPair, ecdsaP256KeyPair, ecdsaP521KeyPair}

	var keys []jose.JSONWebKey
	keys = append(keys, createJSONWebKeys(rsaKeyPair, rsaKid, "RS256", "sig")...)
	keys = append(keys, createJSONWebKeys(ecdsaP256KeyPair, ecdsaP256Kid, "ES256", "sig")...)
	keys = append(keys, createJSONWebKeys(ecdsaP521KeyPair, ecdsaP521Kid, "ES512", "sig")...)

	type args struct {
		ctx context.Context
		set string
	}
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		args    args
		want    *jose.JSONWebKeySet
		wantErr bool
	}{
		{
			name: "With multiple keys per set",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPairs(gomock.Nil(), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(allKeys, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(rsaKeyPair), gomock.Eq(crypto11.CkaId)).Return(pkcs11.NewAttribute(pkcs11.CKA_ID, []byte(rsaKid)), nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(rsaKeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(nil, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(ecdsaP256KeyPair), gomock.Eq(crypto11.CkaId)).Return(pkcs11.NewAttribute(pkcs11.CKA_ID, []byte(ecdsaP256Kid)), nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(ecdsaP256KeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(nil, nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(ecdsaP521KeyPair), gomock.Eq(crypto11.CkaId)).Return(pkcs11.NewAttribute(pkcs11.CKA_ID, []byte(ecdsaP521Kid)), nil)
				hsmContext.EXPECT().GetAttribute(gomock.Eq(ecdsaP521KeyPair), gomock.Eq(crypto11.CkaEncrypt)).Return(nil, nil)
			},
			want: &jose.JSONWebKeySet{Keys: keys},
		},
		{
			name: "Key set not found",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPairs(gomock.Nil(), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(nil, nil)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			m := &hsm.KeyManager{
				Context: hsmContext,
			}
			got, err := m.GetKeySet(tt.args.ctx, tt.args.set)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetKey() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyManager_DeleteKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	hsmContext := NewMockContext(ctrl)
	defer ctrl.Finish()

	rsaKeyPair := NewMockSignerDecrypter(ctrl)

	kid := uuid.New()

	type args struct {
		ctx context.Context
		set string
		kid string
	}
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		args    args
		wantErr bool
	}{
		{
			name: "Existing key",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(rsaKeyPair, nil)
				rsaKeyPair.EXPECT().Delete().Return(nil)
			},
		},
		{
			name: "Key not found",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
				kid: kid,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPair(gomock.Eq([]byte(kid)), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(nil, nil)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			m := &hsm.KeyManager{
				Context: hsmContext,
			}
			if err := m.DeleteKey(tt.args.ctx, tt.args.set, tt.args.kid); (err != nil) != tt.wantErr {
				t.Errorf("DeleteKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKeyManager_DeleteKeySet(t *testing.T) {
	ctrl := gomock.NewController(t)
	hsmContext := NewMockContext(ctrl)
	defer ctrl.Finish()

	rsaKeyPair1 := NewMockSignerDecrypter(ctrl)
	rsaKeyPair2 := NewMockSignerDecrypter(ctrl)
	allKeys := []crypto11.Signer{rsaKeyPair1, rsaKeyPair2}

	type args struct {
		ctx context.Context
		set string
	}
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		args    args
		wantErr bool
	}{
		{
			name: "Existing key",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPairs(gomock.Nil(), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(allKeys, nil)
				rsaKeyPair1.EXPECT().Delete().Return(nil)
				rsaKeyPair2.EXPECT().Delete().Return(nil)
			},
		},
		{
			name: "Key not found",
			args: args{
				ctx: context.TODO(),
				set: x.OpenIDConnectKeyName,
			},
			setup: func(t *testing.T) {
				hsmContext.EXPECT().FindKeyPairs(gomock.Nil(), gomock.Eq([]byte(x.OpenIDConnectKeyName))).Return(nil, nil)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			m := &hsm.KeyManager{
				Context: hsmContext,
			}
			if err := m.DeleteKeySet(tt.args.ctx, tt.args.set); (err != nil) != tt.wantErr {
				t.Errorf("DeleteKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKeyManager_AddKey(t *testing.T) {
	m := &hsm.KeyManager{
		Context: nil,
	}
	err := m.AddKey(context.TODO(), x.OpenIDConnectKeyName, &jose.JSONWebKey{})
	assert.ErrorIs(t, err, hsm.ErrPreGeneratedKeys)
}

func TestKeyManager_AddKeySet(t *testing.T) {
	m := &hsm.KeyManager{
		Context: nil,
	}
	err := m.AddKeySet(context.TODO(), x.OpenIDConnectKeyName, &jose.JSONWebKeySet{})
	assert.ErrorIs(t, err, hsm.ErrPreGeneratedKeys)
}

func TestKeyManager_UpdateKey(t *testing.T) {
	m := &hsm.KeyManager{
		Context: nil,
	}
	err := m.UpdateKey(context.TODO(), x.OpenIDConnectKeyName, &jose.JSONWebKey{})
	assert.ErrorIs(t, err, hsm.ErrPreGeneratedKeys)
}

func TestKeyManager_UpdateKeySet(t *testing.T) {
	m := &hsm.KeyManager{
		Context: nil,
	}
	err := m.UpdateKeySet(context.TODO(), x.OpenIDConnectKeyName, &jose.JSONWebKeySet{})
	assert.ErrorIs(t, err, hsm.ErrPreGeneratedKeys)
}

func expectedKeyAttributes(t *testing.T, kid string) (crypto11.AttributeSet, crypto11.AttributeSet) {
	privateAttrSet, err := crypto11.NewAttributeSetWithIDAndLabel([]byte(kid), []byte(x.OpenIDConnectKeyName))
	require.NoError(t, err)
	publicAttrSet, err := crypto11.NewAttributeSetWithIDAndLabel([]byte(kid), []byte(x.OpenIDConnectKeyName))
	require.NoError(t, err)
	publicAttrSet.AddIfNotPresent([]*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_VERIFY, true),
		pkcs11.NewAttribute(pkcs11.CKA_ENCRYPT, false),
	})
	privateAttrSet.AddIfNotPresent([]*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_SIGN, true),
		pkcs11.NewAttribute(pkcs11.CKA_DECRYPT, false),
	})
	return privateAttrSet, publicAttrSet
}

func expectedKeySet(keyPair *MockSignerDecrypter, kid, alg, use string) *jose.JSONWebKeySet {
	return &jose.JSONWebKeySet{Keys: createJSONWebKeys(keyPair, kid, alg, use)}
}

func createJSONWebKeys(keyPair *MockSignerDecrypter, kid string, alg string, use string) []jose.JSONWebKey {
	return []jose.JSONWebKey{{
		Algorithm:                   alg,
		Use:                         use,
		Key:                         cryptosigner.Opaque(keyPair),
		KeyID:                       fmt.Sprintf("private:%s", kid),
		Certificates:                []*x509.Certificate{},
		CertificateThumbprintSHA1:   []uint8{},
		CertificateThumbprintSHA256: []uint8{},
	}, {
		Algorithm:                   alg,
		Use:                         use,
		Key:                         keyPair.Public(),
		KeyID:                       fmt.Sprintf("public:%s", kid),
		Certificates:                []*x509.Certificate{},
		CertificateThumbprintSHA1:   []uint8{},
		CertificateThumbprintSHA256: []uint8{},
	}}
}