package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gamemini/youtube/pkg/container"
	"github.com/gamemini/youtube/pkg/middleware"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/gorilla/mux"
)

// HandleSubmitVoteWithContainer handles vote submission with dependency injection
func HandleSubmitVoteWithContainer(w http.ResponseWriter, r *http.Request, appContainer *container.AppContainer) {
	vars := mux.Vars(r)
	activityID := vars["id"]
	log.Printf("🗳️ [API] POST /api/activities/%s/vote", activityID)

	var voteRequest models.CreateVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&voteRequest); err != nil {
		log.Printf("❌ [API] Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, _, userID, err := middleware.ExtractUserFromToken(r)
	if err != nil {
		log.Printf("❌ [API] Failed to extract user from token: %v", err)
		http.Error(w, fmt.Sprintf("Authentication required: %v", err), http.StatusUnauthorized)
		return
	}

	log.Printf("🔐 [API] Vote request - UserID: %s, TeamID: %s", userID, voteRequest.TeamID)

	// Use the singleton TeamService from container
	teamService := appContainer.GetTeamService()
	response, err := teamService.SubmitVote(r.Context(), userID, voteRequest.TeamID, activityID)
	if err != nil {
		log.Printf("❌ [API] Failed to submit vote: %v", err)
		http.Error(w, fmt.Sprintf("Failed to submit vote: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("✅ [API] Vote submitted successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    response,
	})
}

// HandleVoteStatusWithContainer handles getting user vote status with dependency injection
func HandleVoteStatusWithContainer(w http.ResponseWriter, r *http.Request, appContainer *container.AppContainer) {
	vars := mux.Vars(r)
	activityID := vars["id"]
	log.Printf("📊 [API] GET /api/activities/%s/vote-status", activityID)

	_, _, userID, err := middleware.ExtractUserFromToken(r)
	if err != nil {
		log.Printf("❌ [API] Failed to extract user from token: %v", err)
		http.Error(w, fmt.Sprintf("Authentication required: %v", err), http.StatusUnauthorized)
		return
	}

	// Use the singleton TeamService from container
	teamService := appContainer.GetTeamService()
	voteStatus, err := teamService.GetUserVoteStatus(r.Context(), userID, activityID)
	if err != nil {
		log.Printf("❌ [API] Failed to get vote status: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get vote status: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [API] Vote status retrieved - HasVoted: %v", voteStatus.HasVoted)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    voteStatus,
	})
}