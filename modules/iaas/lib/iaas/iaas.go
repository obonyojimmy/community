/*
 * Nanocloud Community, a comprehensive platform to turn any application
 * into a cloud solution.
 *
 * Copyright (C) 2015 Nanocloud Software
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

package iaas

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	VMShutdownFailed = errors.New("VM Shutdown Failed")
	VMStartupFailed  = errors.New("VM Startup Failed")
	VMDownloadFailed = errors.New("VM Download Failed")
)

type Iaas struct {
	Server   string
	Password string
	User     string
	SSHPort  string
	InstDir  string
	ArtURL   string
}

type VMstatus struct {
	DownloadingVmNames []string
	AvailableVMNames   []string
	BootingVmNames     []string
	RunningVmNames     []string
}

type VmInfo struct {
	Ico         string
	Name        string
	DisplayName string
	Status      string
	Locked      bool
}

func New(Server, Password, User, SSHPort, InstDir, ArtURL string) *Iaas {
	return &Iaas{
		Server:   Server,
		Password: Password,
		User:     User,
		SSHPort:  SSHPort,
		InstDir:  InstDir,
		ArtURL:   ArtURL,
	}
}

func (i *Iaas) stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func (i *Iaas) CheckVMStates(response VMstatus) []VmInfo {
	var (
		locked      bool
		icon        string
		Status      string
		displayName string
		vmList      []VmInfo
	)

	for _, vmName := range response.AvailableVMNames {

		locked = false
		if strings.Contains(vmName, "windows") {
			if strings.Contains(vmName, "winapps") {
				icon = "settings_applications"
				displayName = "Execution environment"
			} else {
				icon = "windows"
				displayName = "Windows Active Directory"
			}
		} else {
			if strings.Contains(vmName, "drive") {
				icon = "storage"
				displayName = "Drive"
			} else if strings.Contains(vmName, "licence") {
				icon = "vpn_lock"
				displayName = "Windows Licence service"
			} else {
				icon = "apps"
				locked = true
				displayName = "Haptic"
			}
		}

		if i.stringInSlice(vmName, response.RunningVmNames) {
			Status = "running"
		} else if i.stringInSlice(vmName, response.BootingVmNames) {
			Status = "booting"
		} else if i.stringInSlice(vmName, response.DownloadingVmNames) {
			Status = "download"
		} else if i.stringInSlice(vmName, response.AvailableVMNames) {
			Status = "available"
		}
		vmList = append(vmList, VmInfo{
			Ico:         icon,
			Name:        vmName,
			DisplayName: displayName,
			Status:      Status,
			Locked:      locked,
		})
	}
	return vmList
}

func (i *Iaas) CheckRDS() (bool, error) {

	cmd := exec.Command(
		"sshpass", "-p", i.Password,
		"ssh", "-o", "StrictHostKeyChecking=no",
		"-p", i.SSHPort,
		fmt.Sprintf(
			"%s@%s",
			i.User,
			i.Server,
		),
		"C:/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -Command \"Write-Host (Get-Service -Name RDMS).status\"",
	)
	response, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to check windows' state", err, string(response))
		return false, err
	}

	if string(response) == "Running\n" {
		return true, nil
	}
	return false, nil
}

func (i *Iaas) GetList() (VMstatus, error) {
	var status VMstatus

	files, _ := ioutil.ReadDir(fmt.Sprintf("%s/pid/", i.InstDir))
	for _, file := range files {
		fileName := file.Name()
		if !strings.Contains(fileName, ".pid") {
			continue
		}
		running, err := i.CheckRDS()
		if err != nil {
			return status, err
		}
		if running {
			status.RunningVmNames = append(status.RunningVmNames, file.Name()[0:len(file.Name())-4])
		} else {
			status.BootingVmNames = append(status.BootingVmNames, file.Name()[0:len(file.Name())-4])
		}
	}

	files, _ = ioutil.ReadDir(fmt.Sprintf("%s/images/", i.InstDir))
	for _, file := range files {
		fileName := file.Name()
		if !strings.Contains(fileName, ".qcow2") {
			continue
		}
		status.AvailableVMNames = append(status.AvailableVMNames, file.Name()[0:len(file.Name())-6])
	}

	files, _ = ioutil.ReadDir(fmt.Sprintf("%s/downloads/", i.InstDir))
	for _, file := range files {
		fileName := file.Name()
		if !strings.Contains(fileName, ".qcow2") {
			continue
		}
		status.DownloadingVmNames = append(status.DownloadingVmNames, file.Name()[0:len(file.Name())-6])
	}

	return status, nil
}

func (i *Iaas) Stop(name string) error {
	log.Info("stopping : ", name)

	cmd := exec.Command(
		"sshpass", "-p", i.Password,
		"ssh", "-o", "StrictHostKeyChecking=no",
		"-p", i.SSHPort,
		fmt.Sprintf(
			"%s@%s",
			i.User,
			i.Server,
		),
		"C:/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -Command \"Stop-Computer -Force\"",
	)
	response, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to execute sshpass command to shutdown windows", err, string(response))
		return VMShutdownFailed
	}

	return nil
}

func (i *Iaas) Start(name string) error {
	log.Info("Starting : ", name)
	cmd := exec.Command(fmt.Sprintf("%s/scripts/launch-%s.sh", i.InstDir, name))
	err := cmd.Start()
	if err != nil {
		log.Error("Failed to start vm: ", err)
		return VMStartupFailed
	}
	return nil
}

func (i *Iaas) downloadFromUrl(downloadUrl string, dst string) error {
	log.Info("Downloading ", downloadUrl, "to ", dst)

	u, err := url.Parse(downloadUrl)
	if err != nil {
		log.Error("Couldn't parse the VM's URL: ", err)
		return VMDownloadFailed
	}

	splitedPath := strings.Split(u.Path, "/")
	tempDst := filepath.Join(i.InstDir, "downloads", splitedPath[len(splitedPath)-1])
	tmpOutput, err := os.Create(tempDst)
	if err != nil {
		log.Error("Error while creating", tempDst, "-", err)
		return VMDownloadFailed
	}

	response, err := http.Get(downloadUrl)
	if err != nil {
		log.Error("Error while downloading", downloadUrl, "-", err)
		return VMDownloadFailed
	}
	defer response.Body.Close()

	n, err := io.Copy(tmpOutput, response.Body)
	if err != nil {
		log.Error("Error while downloading", downloadUrl, "-", err)
		return VMDownloadFailed
	}
	tmpOutput.Close()

	err = os.Rename(tempDst, dst)
	if err != nil {
		log.Error("Error while creating", dst, "-", err)
		return VMDownloadFailed
	}

	log.Info(n, "bytes downloaded.")
	return nil
}

func (i *Iaas) Download(VMName string) {
	i.downloadFromUrl(
		i.ArtURL+VMName+".qcow2",
		i.InstDir+"/images/"+VMName+".qcow2")
}