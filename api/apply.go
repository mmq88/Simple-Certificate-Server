package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"QuickCertS/data"
	"QuickCertS/model"
	"QuickCertS/utils"

	"github.com/gin-gonic/gin"
)

// Provide the client with a certificate(unique key and signature) for app.
//
// The server currently uses the device information provided by the client.
//
// Check the device info structure in model/device_info.go.
//
// @Summary Provide the client with a certificate(unique key and signature) for app.
// @Description Provide the client with a certificate(unique key and signature) for app.
// @Tags Apply
// @Accept json
// @Produce json
// @Param X-Access-Token header string false "Authorized token for client access. This value is set in path_to_qcs/configs/server.toml."
// @Param applyInfo body model.ApplyCertInfo true "Apply certificate information"
// @Success 200 {object} model.ApplyCertResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /apply/cert [post]
func ApplyCertificate(ctx *gin.Context) {
    applyInfo := model.ApplyCertInfo{}
    err := ctx.ShouldBindJSON(&applyInfo)

    if err != nil {
        ctx.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "Invalid data format."})
        utils.Logger.Error(err.Error())
        return
    }

    excludeList := []string{"Note"} // Allowed empty fields.

    // Check the data is all not empty except for the fields in the excludeList.
    if !utils.IsValidData(applyInfo, excludeList) {
        ctx.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "Invalid data from client."})
        utils.Logger.Error("Invalid data from client.")
        return
    }

    // Check if the SN exists in the database(It's a legal S/N).
    sn_is_exist, err := data.IsSNExist(applyInfo.SerialNumber)

    if err != nil {
        ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Internal server error."})
        return
    }

    if !sn_is_exist {
        ctx.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "The S/N does not exist."})
        utils.Logger.Error(
            fmt.Sprintf("The S/N [%s] does not exist.", applyInfo.SerialNumber),
        )
        return
    }

    // S/N exists, generate a key and a sinature for the device and update it in the database.
    base := fmt.Sprintf("%s&%s&%s&%s&", 
        applyInfo.SerialNumber, applyInfo.BoardProducer, applyInfo.BoardName, applyInfo.MACAddress)
    key, err := data.GetDeviceKeyCache(base)

    // The key not exist in the cache.
    if err != nil {
        if err.Error() == "currently not connecting the redis database" {
            ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
            utils.Logger.Error(err.Error())
            return
        } else {
            // The key not exist in the cache, generate a new key.
            key, err = utils.GenerateKey(base)

            if err != nil {
                ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Internal server error."})
                utils.Logger.Error(err.Error())
                return
            }
            data.SetDeviceKeyCache(base, key)
        }
    }

    if err != nil {
        ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Internal server error."})
        utils.Logger.Error(err.Error())
        return
    }

    signature, err := utils.SignMessage([]byte(key))

    if err != nil {
        ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Internal server error."})
        utils.Logger.Error(err.Error())
        return
    }

    // Update the key corresponding to the SN in the database.
    // If the verification confirms that the key is the same, resend both the key and signature.
    
    if err := data.BindSNWithKey(applyInfo.SerialNumber, key); err != nil {
        if err.Error() == "the s/n does not exist or has already been used" {
            ctx.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
            utils.Logger.Warn(
                fmt.Sprintf("The S/N [%s] does not exist or has already been used.", applyInfo.SerialNumber),
            )
        } else {
			ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
            utils.Logger.Error(err.Error())
        }
        return

    }

    signatureBase64 := base64.StdEncoding.EncodeToString(signature)

    // Sent the certificate to the client.
    ctx.JSON(
        http.StatusOK,
		model.ApplyCertResponse{
			Key:       key,
			Signature: signatureBase64,
		},
    )
    utils.Logger.Info(fmt.Sprintf("Successfully updated and sent the key [%s].", key))
}

// Allow users to apply for temporary use permits on devices.
//
// @Summary Allow users to apply for temporary use permits on devices
// @Description Allow users to apply for temporary use permits on devices.
// @Tags Apply
// @Accept json
// @Produce json
// @Param X-Access-Token header string false "Authorized token for client access. This value is set in path_to_qcs/configs/server.toml."
// @Param applyInfo body model.ApplyTempPermitInfo true "Apply temporary permit information"
// @Success 200 {object} model.ApplyTempPermitResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /apply/temp-permit [post]
func ApplyTemporaryPermit(ctx *gin.Context) {
    applyInfo := model.ApplyTempPermitInfo{}
    err := ctx.ShouldBindJSON(&applyInfo)

    if err != nil {
        ctx.JSON(http.StatusBadRequest, "Invalid data format.")
        utils.Logger.Error(err.Error())
        return
    }

    excludeList := []string{"Note"} // Allowed empty fields.

    // Check the data is all not empty except for the fields in the excludeList.
    if !utils.IsValidData(applyInfo, excludeList) {
        ctx.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "Invalid data from client."})
        utils.Logger.Error("Invalid data from client.")
        return
    }

    // Generate a key for the device and update it in the database.
    base := fmt.Sprintf("%s&%s&%s&%s&", 
        "_", applyInfo.BoardProducer, applyInfo.BoardName, applyInfo.MACAddress)

    key, err := data.GetDeviceKeyCache(base)

    // The key not exist in the cache.
    if err != nil {
        if err.Error() == "currently not connecting the redis database" {
            ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
            utils.Logger.Error(err.Error())
            return
        } else {
            // The key not exist in the cache, generate a new key.
            key, err = utils.GenerateKey(base)

            if err != nil {
                ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Internal server error."})
                utils.Logger.Error(err.Error())
                return
            }
            data.SetDeviceKeyCache(base, key)
        }
    }

    remainingTime, err := data.GetTemporaryPermitExpiredTime(key)

    // The given key has not been used yet, or there is an internal server error.
    if err != nil {
        if strings.Contains(err.Error(), "allowed new key") {
            // Add new key to temporary permit table.
            utils.Logger.Info(err.Error()) // Allowed new key: xxx
            remainingTime, err = data.AddTemporaryPermit(key)

            if err != nil {
                ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
                utils.Logger.Error(err.Error())
            } else {
                ctx.JSON(http.StatusOK, gin.H{
                    "status": "activated",
                    "remaining_time": remainingTime,
                })

                utils.Logger.Info(
                    fmt.Sprintf("Authorized [%s] temporary use of the product remaining [%d s].", key, remainingTime),
                )
            }
            
        } else {
            ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
            utils.Logger.Error(err.Error())
        }
        return
    }

    // Return the remaining valid time.
    if remainingTime > 0 {
        ctx.JSON(
            http.StatusOK,
            gin.H{
                "status": "activated",
                "remaining_time": remainingTime,
            },
        )
        utils.Logger.Info(
            fmt.Sprintf("Authorized [%s] temporary use of the product remaining [%d s].", key, remainingTime),
        )
    } else {
        ctx.JSON(http.StatusOK, model.ErrorResponse{Error: "The authorization has expired."})
        utils.Logger.Info(
            fmt.Sprintf("The authorization for [%s] to use the product has expired.", key),
        )
    }
    
}