package controllers

import (
	"strings"
	"os"
	"net/http"
	"bytes"
	"time"
	"io/ioutil"
	"errors"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/httplib"
	"github.com/astaxie/beego/logs"
	"flux-web/models"
)

type WorkloadController struct {
	beego.Controller
}

var l = logs.GetLogger()

func (this *WorkloadController) ListWorkloads() {
	ns := this.Ctx.Input.Param(":ns")
	l.Printf("in ListWorkloads ns="+ns)
	this.Data["namespaces"] = strings.Split(os.Getenv("NAMESPACES"), ";")
	this.Data["fluxUrl"] = os.Getenv("FLUX_URL")
	this.Data["workloads"] = GetImages(ns, this.Input().Get("filter"))
	this.TplName = "main.tpl"
}

func (this *WorkloadController) ReleaseWorkloads() {
	jobID, err := triggerJob(this.Ctx.Input.RequestBody)
	if err != nil {
		l.Printf(err.Error())
		this.Ctx.Output.SetStatus(500)
		return
	}
	syncID, err := getSyncID(jobID)
	if err != nil {
		l.Printf(err.Error())
		this.Ctx.Output.SetStatus(500)
		return
	}
	this.Ctx.Output.SetStatus(waitForSync(syncID))
}

func waitForSync(syncID string) int{
	l.Printf("waiting for syncID " + syncID)

	for true {
		resp, err := httplib.Get(os.Getenv("FLUX_URL")+"/api/flux/v6/sync?ref="+syncID).String()
		if err != nil {
			l.Printf(err.Error())
			return 500
			break
		}
		if resp == "[]"{
			return 200
			break
		}
		time.Sleep(time.Second)
	}
	 return 500
}

func getSyncID(jobID string) (string, error){
	l.Printf("getting syncID...")

	for true {
		resp, err := httplib.Get(os.Getenv("FLUX_URL")+"/api/flux/v6/jobs?id="+jobID).Bytes()
		if err != nil {
			l.Printf(err.Error())
			return "", errors.New(err.Error())
		}
		job, err := models.NewJob(resp)
		if err != nil {
			l.Panic(err.Error)
			return "", errors.New(err.Error())
		}
		if job.Result.Revision != "" {
			l.Printf(job.Result.Revision)
			return job.Result.Revision,nil
		} else if job.Err != "" {
			l.Printf("Error: " + job.Err)
			return job.Err, errors.New(job.Err)
		} else {
			l.Printf("job status: " + job.StatusString)
		}
		time.Sleep(time.Second)
	}
	 return "", nil
}

func triggerJob(requestBody []byte) (string, error){
	resp, err := http.Post(os.Getenv("FLUX_URL")+"/api/flux/v9/update-manifests", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		l.Printf(err.Error())
		return "", errors.New(err.Error())
	}
	defer resp.Body.Close();
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		l.Panic(err.Error)
		return "", errors.New(err.Error())
	}
	jobID := strings.Replace(string(body), "\"", "", -1)
	l.Printf("job "+ jobID + " triggered")

	return string(jobID), nil
}

func GetImages(params ...string) []models.Image{
	namespace := os.Getenv("DEFAULT_NAMESPACE")
	if len(params) > 0 {
		namespace = params[0]
		l.Printf(namespace)
	}
	res, err := httplib.Get(os.Getenv("FLUX_URL")+"/api/flux/v10/images?namespace="+namespace).Debug(true).Bytes()
	if err != nil {
		l.Panic(err.Error)
	}

	images, err := models.NewImages(res)
	if err != nil {
		l.Panic(err.Error)
	}
	if len(params) > 1 {
		filter := params[1]
		l.Printf(filter)
		for i := 0; i < len(images); i++ {
			if !strings.Contains(images[i].ID, filter) {
				images = append(images[:i], images[i+1:]...)
				i-- 
			}
		}
	}
	return images
}