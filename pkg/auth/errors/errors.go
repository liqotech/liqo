// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
