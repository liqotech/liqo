package errors

// ClientError is returned when the error is caused by clients bad values.
type ClientError struct {
	Reason string
}

func (err *ClientError) Error() string {
	return err.Reason
}

// AuthenticationFailedError is returned when the auth service was not able to validate the user pre-authentication.
type AuthenticationFailedError struct {
	Reason string
}

func (err *AuthenticationFailedError) Error() string {
	return err.Reason
}
