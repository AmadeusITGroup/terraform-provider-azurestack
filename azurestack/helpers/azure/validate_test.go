package azure

import "testing"

func TestHelper_AzureResourceID(t *testing.T) {
	cases := []struct {
		ID     string
		Errors int
	}{
		{
			ID:     "",
			Errors: 1,
		},
		{
			ID:     "nonsense",
			Errors: 1,
		},
		{
			ID:     "/slash",
			Errors: 1,
		},
		{
			ID:     "/path/to/nothing",
			Errors: 1,
		},
		{
			ID:     "/subscriptions",
			Errors: 1,
		},
		{
			ID:     "/providers",
			Errors: 1,
		},
		{
			ID:     "/subscriptions/not-a-guid",
			Errors: 0,
		},
		{
			ID:     "/providers/test",
			Errors: 0,
		},
		{
			ID:     "/subscriptions/00000000-0000-0000-0000-00000000000/",
			Errors: 0,
		},
		{
			ID:     "/providers/provider.name/",
			Errors: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			_, errors := ValidateResourceID(tc.ID, "test")

			if len(errors) < tc.Errors {
				t.Fatalf("Expected ValidateResourceID to have %d not %d errors for %q", tc.Errors, len(errors), tc.ID)
			}
		})
	}
}

func TestAzureResourceIDOrEmpty(t *testing.T) {
	cases := []struct {
		ID     string
		Errors int
	}{
		{
			ID:     "",
			Errors: 0,
		},
		{
			ID:     "nonsense",
			Errors: 1,
		},
		//as this function just calls TestAzureResourceId lets not be as comprehensive
		{
			ID:     "/providers/provider.name/",
			Errors: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			_, errors := ValidateResourceIDOrEmpty(tc.ID, "test")

			if len(errors) < tc.Errors {
				t.Fatalf("Expected TestAzureResourceIdOrEmpty to have %d not %d errors for %q", tc.Errors, len(errors), tc.ID)
			}
		})
	}
}

func TestValidateVaultName(t *testing.T) {
	cases := []struct {
		Input       string
		ExpectError bool
	}{
		{
			Input:       "",
			ExpectError: true,
		},
		{
			Input:       "hi",
			ExpectError: true,
		},
		{
			Input:       "hello",
			ExpectError: false,
		},
		{
			Input:       "hello-world",
			ExpectError: false,
		},
		{
			Input:       "hello-world-21",
			ExpectError: false,
		},
		{
			Input:       "hello_world_21",
			ExpectError: true,
		},
		{
			Input:       "Hello-World",
			ExpectError: false,
		},
		{
			Input:       "20202020",
			ExpectError: false,
		},
		{
			Input:       "ABC123!@£",
			ExpectError: true,
		},
		{
			Input:       "abcdefghijklmnopqrstuvwxyz",
			ExpectError: true,
		},
	}

	for _, tc := range cases {
		_, errors := VaultName(tc.Input, "")

		hasError := len(errors) > 0
		if tc.ExpectError && !hasError {
			t.Fatalf("Expected the Key Vault Name to trigger a validation error for '%s'", tc.Input)
		}
	}
}
