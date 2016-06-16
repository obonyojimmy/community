/*
 * Nanocloud Community, a comprehensive platform to turn any application
 * into a cloud solution.
 *
 * Copyright (C) 2015 Nanocloud Software
 *
 * This file is part of Nanocloud community.
 *
 * Nanocloud community is free software; you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * Nanocloud community is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package users

import (
	"errors"
	"net/http"

	apiErrors "github.com/Nanocloud/community/nanocloud/errors"
	"github.com/Nanocloud/community/nanocloud/models/ldap"
	"github.com/Nanocloud/community/nanocloud/models/users"
	"github.com/Nanocloud/community/nanocloud/utils"
	log "github.com/Sirupsen/logrus"
	"github.com/labstack/echo"
)

type hash map[string]interface{}

func Delete(c *echo.Context) error {
	userId := c.Param("id")
	if len(userId) == 0 {
		return c.JSON(http.StatusBadRequest, hash{
			"error": [1]hash{
				hash{
					"detail": "User id needed for deletion",
				},
			},
		})
	}

	user, err := users.GetUser(userId)
	if err != nil {
		return err
	}

	if user == nil {
		return c.JSON(http.StatusNotFound, hash{
			"error": [1]hash{
				hash{
					"detail": "User not found",
				},
			},
		})
	}

	if user.IsAdmin {
		return c.JSON(http.StatusUnauthorized, hash{
			"error": [1]hash{
				hash{
					"detail": "Admins cannot be deleted",
				},
			},
		})
	}

	err = ldap.DeleteAccount(user.Id)
	if err != nil {
		log.Errorf("Unable to delete user in ad: %s", err.Error())
		switch err {
		case ldap.DeleteFailed:
			return c.JSON(http.StatusInternalServerError, hash{
				"error": [1]hash{
					hash{
						"detail": err.Error(),
					},
				},
			})
		case ldap.UnknownUser:
			log.Info("User doesn't exist in AD")
			break
		default:
			return err
		}
	}

	err = users.DeleteUser(user.Id)
	if err != nil {
		log.Errorf("Unable to delete user: ", err.Error())
		return err
	}

	return c.JSON(http.StatusOK, hash{
		"meta": hash{},
	})
}

func Disable(userId string) (int, error) {
	if userId == "" {
		return http.StatusNotFound, errors.New("User id needed for desactivation")
	}

	exists, err := users.UserExists(userId)
	if err != nil {
		return http.StatusConflict, err
	}

	if !exists {
		return http.StatusNotFound, errors.New("User not found")
	}

	err = users.DisableUser(userId)
	if err != nil {
		return http.StatusInternalServerError, errors.New("Unable to disable user: " + err.Error())
	}

	return 0, nil
}

func Update(c *echo.Context) error {
	u := users.User{}
	uid := c.Param("id")
	u.SetID(uid)
	err := utils.ParseJSONBody(c, &u)
	if err != nil {
		return apiErrors.InvalidRequest
	}

	user, err := users.GetUser(u.GetID())
	if err != nil {
		return apiErrors.UserNotFound
	}

	if u.IsAdmin != user.IsAdmin {
		if u.Id == user.Id {
			return apiErrors.Unauthorized.Detail("You can't change your own rank")
		}
		err = users.UpdateUserRank(user.GetID(), u.IsAdmin)
		if err != nil {
			log.Error(err)
			return apiErrors.InternalError.Detail("Unable to update the rank.")
		}
	} else if u.Password != "" {
		err = users.UpdateUserPassword(user.GetID(), u.Password)
		if err != nil {
			log.Error(err)
			return apiErrors.InternalError.Detail("Unable to update the password.")
		}
	} else if u.Email != user.Email {
		err = users.UpdateUserEmail(user.GetID(), u.Email)
		if err != nil {
			log.Error(err)
			return apiErrors.InternalError.Detail("Unable to update the email.")
		}
	} else if u.FirstName != user.FirstName {
		err = users.UpdateUserFirstName(user.GetID(), u.FirstName)
		if err != nil {
			log.Error(err)
			return apiErrors.InternalError.Detail("Unable to update the first name.")
		}
	} else if u.LastName != user.LastName {
		err = users.UpdateUserLastName(user.GetID(), u.LastName)
		if err != nil {
			log.Error(err)
			return apiErrors.InternalError.Detail("Unable to update the last name.")
		}
	} else {
		return apiErrors.InvalidRequest.Detail("No field send.")
	}

	return utils.JSON(c, http.StatusOK, user)
}

func Get(c *echo.Context) error {
	user := c.Get("user").(*users.User)
	if c.Query("me") == "true" {
		return utils.JSON(c, http.StatusOK, user)
	}

	if !user.IsAdmin {
		return apiErrors.AdminLevelRequired
	}

	users, err := users.FindUsers()
	if err != nil {
		log.Error(err)
		return apiErrors.InternalError.Detail("Unable to retreive the user list.")
	}

	return utils.JSON(c, http.StatusOK, users)
}

func Post(c *echo.Context) error {
	u := users.User{}

	err := utils.ParseJSONBody(c, &u)
	if err != nil {
		return err
	}

	if u.Email == "" {
		return c.JSON(http.StatusBadRequest, hash{
			"error": [1]hash{
				hash{
					"detail": "email is missing",
				},
			},
		})
	}

	if u.FirstName == "" {
		return c.JSON(http.StatusBadRequest, hash{
			"error": [1]hash{
				hash{
					"detail": "first-name is missing",
				},
			},
		})
	}

	if u.LastName == "" {
		return c.JSON(http.StatusBadRequest, hash{
			"error": [1]hash{
				hash{
					"detail": "last-name is missing",
				},
			},
		})
	}

	if u.Password == "" {
		return c.JSON(http.StatusBadRequest, hash{
			"error": [1]hash{
				hash{
					"detail": "password is missing",
				},
			},
		})
	}

	newUser, err := users.CreateUser(
		true,
		u.Email,
		u.FirstName,
		u.LastName,
		u.Password,
		false,
	)
	switch err {
	case users.UserDuplicated:
		return c.JSON(http.StatusConflict, hash{
			"error": [1]hash{
				hash{
					"detail": err.Error(),
				},
			},
		})
	case users.UserNotCreated:
		return err
	}

	winpass := utils.RandomString(8) + "s4D+"
	sam, err := ldap.AddUser(newUser.Id, winpass)
	if err != nil {
		return err
	}

	err = users.UpdateUserAd(newUser.Id, sam, winpass, "intra.localdomain.com")
	if err != nil {
		return err
	}

	return utils.JSON(c, http.StatusCreated, newUser)
}

func UpdatePassword(c *echo.Context) error {
	userId := c.Param("id")
	if userId == "" {
		return c.JSON(http.StatusBadRequest, hash{
			"error": [1]hash{
				hash{
					"detail": "User id needed to modify account",
				},
			},
		})
	}

	var user struct {
		Data struct {
			Password string
		}
	}

	err := utils.ParseJSONBody(c, &user)
	if err != nil {
		return nil
	}

	exists, err := users.UserExists(userId)
	if err != nil {
		log.Errorf("Unable to check user existance: %s", err.Error())
		return err
	}

	if !exists {
		return c.JSON(http.StatusNotFound, hash{
			"error": [1]hash{
				hash{
					"detail": "User not found",
				},
			},
		})
	}

	err = users.UpdateUserPassword(userId, user.Data.Password)
	if err != nil {
		log.Errorf("Unable to update user password: %s", err.Error())
		return err
	}

	return c.JSON(http.StatusOK, hash{
		"data": hash{
			"success": true,
		},
	})
}

func GetUser(c *echo.Context) error {
	userId := c.Param("id")
	if userId == "" {
		return c.JSON(http.StatusBadRequest, hash{
			"error": [1]hash{
				hash{
					"detail": "User id needed to retrieve account informations",
				},
			},
		})
	}

	user, err := users.GetUser(userId)
	if err != nil {
		return err
	}

	if user == nil {
		return c.JSON(http.StatusNotFound, hash{
			"error": [1]hash{
				hash{
					"detail": "User Not Found",
				},
			},
		})
	}

	return utils.JSON(c, http.StatusOK, user)
}
