# YouTube Data API v3 Setup Guide

This guide explains how to set up YouTube Data API v3 with service account authentication for the YouTube activity platform backend.

## Problem Solved

The backend was previously trying to use custom JWT tokens as Google OAuth tokens for YouTube API calls, which failed with 401 authentication errors. This implementation now uses YouTube Data API v3 with API key authentication.

## API Key Setup

### 1. Google Cloud Console Setup

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Select your project or create a new one
3. Enable the YouTube Data API v3:
   - Navigate to "APIs & Services" → "Library"
   - Search for "YouTube Data API v3"
   - Click "Enable"

### 2. Create API Key

1. Go to "APIs & Services" → "Credentials"
2. Click "Create Credentials" → "API Key"
3. Copy the generated API key
4. (Recommended) Restrict the API key:
   - Click on the API key to edit
   - Under "API restrictions", select "Restrict key"
   - Choose "YouTube Data API v3"
   - Under "Application restrictions", configure as needed

### 3. Configure Environment Variable

Replace the dummy API key in `.env.local`:

```bash
# Replace this dummy key with your real API key
YOUTUBE_API_KEY=YOUR_ACTUAL_YOUTUBE_API_KEY_HERE
```

## Implementation Details

### New Functions Added

1. **`createYouTubeServiceWithAPIKey`**: Creates YouTube service using API key instead of OAuth
2. **`checkSubscriptionWithAPIKey`**: Attempts subscription verification with API key (limited)
3. **`getUserChannelIDFromToken`**: Extracts user channel ID from OAuth tokens
4. **`handleYouTubeAPIError`**: Enhanced error handling for API key-specific errors

### Key Changes

- **Service Creation**: Now uses `option.WithAPIKey()` instead of OAuth token source
- **Error Handling**: Added specific handling for API key errors (invalid key, quota exceeded, etc.)
- **Logging**: Comprehensive logging for debugging API key authentication
- **Fallback Approach**: Graceful handling when subscription verification isn't possible with API key

## API Limitations

### Subscription Verification Challenges

**Important**: YouTube Data API v3 with API key authentication has limitations:

1. **Cannot use `mine=true`**: API key auth doesn't support user-specific queries
2. **Subscription privacy**: User subscription lists are often private
3. **OAuth required**: True subscription verification requires OAuth user consent

### Current Implementation Strategy

The implementation provides:

1. **Channel verification**: Confirms target channels exist
2. **Fallback logic**: For demonstration/testing purposes
3. **Comprehensive logging**: For debugging and monitoring
4. **Error handling**: Graceful degradation when verification fails

### Production Recommendations

For production use, consider these approaches:

1. **Hybrid Authentication**:
   ```go
   // Use API key for public data (channel info)
   channelService := createYouTubeServiceWithAPIKey(ctx, apiKey)
   
   // Use OAuth for user-specific data (subscriptions)
   userService := createYouTubeService(ctx, oauthToken)
   ```

2. **Webhook Verification**: Implement YouTube webhook subscriptions
3. **User-Initiated Verification**: Let users prove subscription through UI flow
4. **Alternative Methods**: Use channel membership or other verification approaches

## Testing

### 1. Environment Setup

```bash
# Ensure API key is set
export YOUTUBE_API_KEY="your_actual_api_key"

# Start the backend
go run main.go
```

### 2. Test Endpoints

**Check Subscription (General)**:
```bash
curl -X GET "http://localhost:8080/api/check-subscription?channel_id=UC_TARGET_CHANNEL_ID" \
  -H "Authorization: Bearer your_token_here"
```

**Annanped Subscription Check**:
```bash
curl -X GET "http://localhost:8080/api/annanped/subscription-check" \
  -H "Authorization: Bearer your_token_here"
```

### 3. Expected Behavior

- **With valid API key**: Channel verification works, subscription check has limitations
- **With invalid API key**: Clear error messages about API configuration
- **Without API key**: Graceful error handling

### 4. Log Analysis

Check logs for:
- `✅ [YOUTUBE-SERVICE-API] YouTube service created successfully`
- `🔑 [SUBSCRIPTION-CHECK] YouTube API key found`
- Error patterns for troubleshooting

## Security Considerations

### API Key Security

1. **Environment Variables**: Never commit API keys to version control
2. **Restrictions**: Apply appropriate API key restrictions in Google Cloud Console
3. **Rotation**: Regularly rotate API keys
4. **Monitoring**: Monitor API usage for unusual patterns

### Rate Limiting

YouTube Data API v3 has quotas:
- **Default quota**: 10,000 units per day
- **Channel list**: 1 unit per request
- **Subscription check**: 1 unit per request

Monitor usage in Google Cloud Console.

## Troubleshooting

### Common Issues

1. **"Invalid API key"**:
   - Verify API key is correctly set in `.env.local`
   - Check API key restrictions in Google Cloud Console
   - Ensure YouTube Data API v3 is enabled

2. **"Quota exceeded"**:
   - Check quota usage in Google Cloud Console
   - Implement caching to reduce API calls
   - Request quota increase if needed

3. **"Channel not found"**:
   - Verify channel ID format (should start with "UC")
   - Check if channel exists and is public

### Debug Commands

```bash
# Check environment loading
grep YOUTUBE_API_KEY .env.local

# Test API key directly
curl "https://www.googleapis.com/youtube/v3/channels?part=snippet&id=UCdummy&key=YOUR_API_KEY"

# Check application logs
go run main.go 2>&1 | grep -i youtube
```

## Migration from OAuth

If migrating from OAuth-based implementation:

1. **Preserve OAuth functions**: Keep existing functions for backward compatibility
2. **Add API key alternatives**: New functions use API key authentication
3. **Update calling code**: Gradually migrate to API key where appropriate
4. **Test thoroughly**: Ensure both approaches work during transition

## Future Enhancements

Potential improvements:

1. **Caching**: Implement Redis caching for channel data
2. **Batch requests**: Combine multiple channel checks
3. **Webhook integration**: Real-time subscription updates
4. **Analytics**: Track subscription verification patterns
5. **Multi-key support**: Load balancing across multiple API keys