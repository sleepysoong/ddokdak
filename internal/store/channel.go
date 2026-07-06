// Package store provides storage implementations for the ddokdak Discord bot.
package store

import (
	"fmt"
	"sync"
)

// ChannelStore defines the interface for managing registered channels per guild.
type ChannelStore interface {
	// AddChannel registers a channel for a given guild.
	AddChannel(guildID, channelID string) error

	// RemoveChannel unregisters a channel from a given guild.
	RemoveChannel(guildID, channelID string) error

	// IsRegistered checks whether a channel is registered for a given guild.
	IsRegistered(guildID, channelID string) bool

	// GetChannels returns all registered channel IDs for a given guild.
	GetChannels(guildID string) []string
}

// InMemoryChannelStore is a concurrency-safe, in-memory implementation of ChannelStore.
type InMemoryChannelStore struct {
	mu       sync.RWMutex
	channels map[string]map[string]struct{}
}

// NewInMemoryChannelStore creates and returns a new InMemoryChannelStore.
func NewInMemoryChannelStore() *InMemoryChannelStore {
	return &InMemoryChannelStore{
		channels: make(map[string]map[string]struct{}),
	}
}

// AddChannel registers a channel for a given guild.
// Returns an error if the channel is already registered.
func (s *InMemoryChannelStore) AddChannel(guildID, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.channels[guildID]; !ok {
		s.channels[guildID] = make(map[string]struct{})
	}

	if _, exists := s.channels[guildID][channelID]; exists {
		return fmt.Errorf("channel %s is already registered in guild %s", channelID, guildID)
	}

	s.channels[guildID][channelID] = struct{}{}
	return nil
}

// RemoveChannel unregisters a channel from a given guild.
// Returns an error if the channel is not registered.
func (s *InMemoryChannelStore) RemoveChannel(guildID, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	guildChannels, ok := s.channels[guildID]
	if !ok {
		return fmt.Errorf("channel %s is not registered in guild %s", channelID, guildID)
	}

	if _, exists := guildChannels[channelID]; !exists {
		return fmt.Errorf("channel %s is not registered in guild %s", channelID, guildID)
	}

	delete(guildChannels, channelID)

	if len(guildChannels) == 0 {
		delete(s.channels, guildID)
	}

	return nil
}

// IsRegistered checks whether a channel is registered for a given guild.
func (s *InMemoryChannelStore) IsRegistered(guildID, channelID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	guildChannels, ok := s.channels[guildID]
	if !ok {
		return false
	}

	_, exists := guildChannels[channelID]
	return exists
}

// GetChannels returns all registered channel IDs for a given guild.
// Returns an empty slice if no channels are registered.
func (s *InMemoryChannelStore) GetChannels(guildID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	guildChannels, ok := s.channels[guildID]
	if !ok {
		return []string{}
	}

	result := make([]string, 0, len(guildChannels))
	for id := range guildChannels {
		result = append(result, id)
	}

	return result
}
