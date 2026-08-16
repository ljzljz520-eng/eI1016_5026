package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"sync"
	"sync/atomic"
)

type LoginResult string

const (
	LoginAllowed LoginResult = "allowed"
	LoginDenied  LoginResult = "denied"
	LoginLocked  LoginResult = "locked"
)

type Account struct {
	Username     string
	PasswordHash [sha256.Size]byte
}

func NewAccount(username, password string) Account {
	return Account{Username: username, PasswordHash: sha256.Sum256([]byte(password))}
}

type AccountRepository interface {
	Find(username string) (Account, bool)
}

type ProtectionRepository interface {
	IsLocked(username string) bool
	RecordFailure(username string) ProtectionState
	Reset(username string)
}

type ProtectionState struct {
	Failures int
	Locked   bool
}

type Service struct {
	accounts   AccountRepository
	protection ProtectionRepository
}

func NewService(accounts AccountRepository, protection ProtectionRepository) *Service {
	return &Service{accounts: accounts, protection: protection}
}

func (s *Service) Login(username, password string) LoginResult {
	account, ok := s.accounts.Find(username)
	if !ok {
		return LoginDenied
	}
	if s.protection.IsLocked(username) {
		return LoginLocked
	}
	candidate := sha256.Sum256([]byte(password))
	if subtle.ConstantTimeCompare(account.PasswordHash[:], candidate[:]) == 1 {
		s.protection.Reset(username)
		return LoginAllowed
	}
	state := s.protection.RecordFailure(username)
	if state.Locked {
		return LoginLocked
	}
	return LoginDenied
}

type MemoryAccountRepository struct {
	accounts map[string]Account
}

func NewMemoryAccountRepository(accounts []Account) *MemoryAccountRepository {
	items := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		items[account.Username] = account
	}
	return &MemoryAccountRepository{accounts: items}
}

func (r *MemoryAccountRepository) Find(username string) (Account, bool) {
	account, ok := r.accounts[username]
	return account, ok
}

type Barrier struct {
	mu           sync.Mutex
	participants int
	arrived      int
	release      chan struct{}
}

func NewBarrier(participants int) *Barrier {
	if participants < 1 {
		participants = 1
	}
	return &Barrier{participants: participants, release: make(chan struct{})}
}

func (b *Barrier) Wait() {
	b.mu.Lock()
	if b.arrived >= b.participants {
		b.mu.Unlock()
		return
	}
	b.arrived++
	release := b.release
	if b.arrived == b.participants {
		close(release)
	}
	b.mu.Unlock()
	<-release
}

type failureState struct {
	failures atomic.Int64
	locked   atomic.Bool
}

type MemoryProtectionRepository struct {
	states    map[string]*failureState
	threshold int64
	barrier   *Barrier
}

func NewMemoryProtectionRepository(usernames []string, threshold int, barrier *Barrier) *MemoryProtectionRepository {
	states := make(map[string]*failureState, len(usernames))
	for _, username := range usernames {
		states[username] = &failureState{}
	}
	return &MemoryProtectionRepository{
		states:    states,
		threshold: int64(threshold),
		barrier:   barrier,
	}
}

func (r *MemoryProtectionRepository) IsLocked(username string) bool {
	state, ok := r.states[username]
	return ok && state.locked.Load()
}

func (r *MemoryProtectionRepository) RecordFailure(username string) ProtectionState {
	state, ok := r.states[username]
	if !ok {
		return ProtectionState{}
	}
	failures := state.failures.Load()
	if r.barrier != nil {
		r.barrier.Wait()
	}
	failures++
	state.failures.Store(failures)
	if failures >= r.threshold {
		state.locked.Store(true)
	}
	return ProtectionState{Failures: int(failures), Locked: state.locked.Load()}
}

func (r *MemoryProtectionRepository) Reset(username string) {
	state, ok := r.states[username]
	if !ok {
		return
	}
	state.failures.Store(0)
}
