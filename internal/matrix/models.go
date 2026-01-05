package matrix

// User represents a Matrix user
type User struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"displayname,omitempty"`
	Admin       bool   `json:"admin,omitempty"`
	Deactivated bool   `json:"deactivated,omitempty"`
}

// CreateUserRequest is the request body for creating a user via Admin API
type CreateUserRequest struct {
	Password    string `json:"password,omitempty"`
	DisplayName string `json:"displayname,omitempty"`
	Admin       bool   `json:"admin"`
	Deactivated bool   `json:"deactivated"`
}

// UserResponse is the response from the Admin API for user operations
type UserResponse struct {
	Name        string `json:"name"`
	UserID      string `json:"user_id,omitempty"`
	DisplayName string `json:"displayname,omitempty"`
	Admin       bool   `json:"admin"`
	Deactivated bool   `json:"deactivated"`
	Errcode     string `json:"errcode,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Room represents a Matrix room
type Room struct {
	RoomID       string   `json:"room_id"`
	Name         string   `json:"name,omitempty"`
	Topic        string   `json:"topic,omitempty"`
	CanonicalAlias string `json:"canonical_alias,omitempty"`
	JoinedMembers int     `json:"joined_members,omitempty"`
	Creator      string   `json:"creator,omitempty"`
	Public       bool     `json:"public,omitempty"`
}

// CreateRoomRequest is the request body for creating a room
type CreateRoomRequest struct {
	Name            string                 `json:"name,omitempty"`
	Topic           string                 `json:"topic,omitempty"`
	RoomAliasName   string                 `json:"room_alias_name,omitempty"`
	Visibility      string                 `json:"visibility,omitempty"`
	Preset          string                 `json:"preset,omitempty"`
	IsDirect        bool                   `json:"is_direct,omitempty"`
	CreationContent map[string]interface{} `json:"creation_content,omitempty"`
	InitialState    []StateEvent           `json:"initial_state,omitempty"`
	Invite          []string               `json:"invite,omitempty"`
}

// CreateRoomResponse is the response from creating a room
type CreateRoomResponse struct {
	RoomID  string `json:"room_id,omitempty"`
	Errcode string `json:"errcode,omitempty"`
	Error   string `json:"error,omitempty"`
}

// StateEvent represents a Matrix state event
type StateEvent struct {
	Type     string      `json:"type"`
	StateKey string      `json:"state_key"`
	Content  interface{} `json:"content"`
}

// SpaceChildContent is the content for m.space.child events
type SpaceChildContent struct {
	Via       []string `json:"via,omitempty"`
	Suggested bool     `json:"suggested,omitempty"`
	Order     string   `json:"order,omitempty"`
}

// InviteRequest is the request body for inviting a user to a room
type InviteRequest struct {
	UserID string `json:"user_id"`
}

// JoinRequest is the request body for joining a room
type JoinRequest struct {
	Reason string `json:"reason,omitempty"`
}

// GenericResponse is a generic API response
type GenericResponse struct {
	Errcode string `json:"errcode,omitempty"`
	Error   string `json:"error,omitempty"`
}

// WhoAmIResponse is the response from the whoami endpoint
type WhoAmIResponse struct {
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id,omitempty"`
	Errcode  string `json:"errcode,omitempty"`
	Error    string `json:"error,omitempty"`
}

// SpaceParentContent is the content for m.space.parent events
type SpaceParentContent struct {
	Via       []string `json:"via,omitempty"`
	Canonical bool     `json:"canonical,omitempty"`
}

// RoomNameContent is the content for m.room.name events
type RoomNameContent struct {
	Name string `json:"name"`
}

// RoomTopicContent is the content for m.room.topic events
type RoomTopicContent struct {
	Topic string `json:"topic"`
}

// ImportResult represents the result of an import operation
type ImportResult struct {
	UserID       string `json:"user_id,omitempty"`
	RoomID       string `json:"room_id,omitempty"`
	SpaceID      string `json:"space_id,omitempty"`
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	AlreadyExists bool  `json:"already_exists,omitempty"`
}

// ImportStats holds statistics about an import operation
type ImportStats struct {
	UsersCreated    int `json:"users_created"`
	UsersSkipped    int `json:"users_skipped"`
	UsersFailed     int `json:"users_failed"`
	SpacesCreated   int `json:"spaces_created"`
	SpacesSkipped   int `json:"spaces_skipped"`
	SpacesFailed    int `json:"spaces_failed"`
	RoomsCreated    int `json:"rooms_created"`
	RoomsSkipped    int `json:"rooms_skipped"`
	RoomsFailed     int `json:"rooms_failed"`
	MembersAdded    int `json:"members_added"`
	MembersSkipped  int `json:"members_skipped"`
	MembersFailed   int `json:"members_failed"`
	RoomsLinked     int `json:"rooms_linked"`
	RoomsLinkFailed int `json:"rooms_link_failed"`
}

// RoomPreset defines room creation presets
type RoomPreset string

const (
	PresetPrivateChat        RoomPreset = "private_chat"
	PresetPublicChat         RoomPreset = "public_chat"
	PresetTrustedPrivateChat RoomPreset = "trusted_private_chat"
)

// RoomVisibility defines room visibility options
type RoomVisibility string

const (
	VisibilityPublic  RoomVisibility = "public"
	VisibilityPrivate RoomVisibility = "private"
)

// SpaceType is the type identifier for spaces
const SpaceType = "m.space"

// EventTypes
const (
	EventTypeSpaceChild  = "m.space.child"
	EventTypeSpaceParent = "m.space.parent"
	EventTypeRoomName    = "m.room.name"
	EventTypeRoomTopic   = "m.room.topic"
)



