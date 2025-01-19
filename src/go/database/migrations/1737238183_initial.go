package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	maxTextLength = 1_000_000_000 // 1 GB
	maxFileSize   = 1_000_000_000 // 1 GB
)

func updateAppSettings(app core.App) error {
	settings := app.Settings()

	settings.Meta.AppName = "IMAP Backup"

	if err := app.Save(settings); err != nil {
		return fmt.Errorf("failed to save app settings: %w", err)
	}

	return nil
}

func updatePbUsersAuth(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("_pb_users_auth_")
	if err != nil {
		return err
	}

	collection.CreateRule = nil

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to update '_pb_users_auth_' collection: %w", err)
	}

	return nil
}

func createSmtpAccounts(app core.App) error {
	collection := core.NewCollection("base", "smtp_accounts")
	collection.Id = "ib_smtp_accounts"

	collection.ListRule = types.Pointer("created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "created_by",
			CollectionId: "_pb_users_auth_",
			MinSelect:    1,
			MaxSelect:    1,
			Required:     true,
		},
		&core.TextField{
			Name:        "username",
			Presentable: true,
			Required:    true,
		},
		&core.TextField{
			Name:     "password",
			Required: true,
			Hidden:   true,
		},
		&core.TextField{
			Name:        "host",
			Presentable: true,
			Required:    true,
		},
		&core.NumberField{
			Name:        "port",
			Min:         types.Pointer(1.0),
			Max:         types.Pointer(65535.0),
			OnlyInt:     true,
			Presentable: true,
			Required:    true,
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	collection.AddIndex("idx_ib_smtp_accounts_created_by", false, "`created_by`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'smtp_accounts' collection: %w", err)
	}

	return nil
}

func createEmails(app core.App) error {
	collection := core.NewCollection("base", "emails")
	collection.Id = "ib_emails"

	collection.ListRule = types.Pointer("smtp_account.created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("smtp_account.created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("smtp_account.created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("smtp_account.created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "smtp_account",
			CollectionId: "ib_smtp_accounts",
			MinSelect:    1,
			MaxSelect:    1,
			Presentable:  true,
			Required:     true,
		},
		&core.BoolField{
			Name: "inserted",
		},
		&core.TextField{
			Name:        "folder",
			Presentable: true,
		},
		&core.DateField{
			Name: "received",
		},
		&core.DateField{
			Name: "sent",
		},
		&core.NumberField{
			Name:    "size",
			Min:     types.Pointer(0.0),
			OnlyInt: true,
		},
		&core.TextField{
			Name:        "subject",
			Presentable: true,
		},
		&core.NumberField{
			Name:    "uid",
			OnlyInt: true,
		},
		&core.TextField{
			Name: "message_id",
		},
		&core.TextField{
			Name: "text",
			Max:  maxTextLength,
		},
		&core.TextField{
			Name: "html",
			Max:  maxTextLength,
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	collection.AddIndex("idx_ib_emails_folder", false, "`smtp_account`,`inserted`,`folder`", "")
	collection.AddIndex("idx_ib_emails_uid", false, "`smtp_account`,`inserted`,`uid`", "")
	collection.AddIndex("idx_ib_emails_message_id", false, "`smtp_account`,`inserted`,`message_id`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'emails' collection: %w", err)
	}

	return nil
}

func createEmailFlags(app core.App) error {
	collection := core.NewCollection("base", "email_flags")
	collection.Id = "ib_email_flags"

	collection.ListRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "email",
			CollectionId: "ib_emails",
			MinSelect:    1,
			MaxSelect:    1,
			Required:     true,
		},
		&core.NumberField{
			Name:        "index",
			Presentable: true,
			Min:         types.Pointer(0.0),
			OnlyInt:     true,
		},
		&core.TextField{
			Name:        "flag",
			Presentable: true,
			Max:         maxTextLength,
		},
	)

	collection.AddIndex("idx_ib_email_flags_email_index", false, "`email`,`index`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'email_flags' collection: %w", err)
	}

	return nil
}

func createEmailFromAddresses(app core.App) error {
	collection := core.NewCollection("base", "email_from_addresses")
	collection.Id = "ib_email_from_addresses"

	collection.ListRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "email",
			CollectionId: "ib_emails",
			MinSelect:    1,
			MaxSelect:    1,
			Required:     true,
		},
		&core.EmailField{
			Name:        "email_address",
			Presentable: true,
		},
		&core.TextField{
			Name:        "display_name",
			Presentable: true,
		},
	)

	collection.AddIndex("idx_ib_email_from_addresses_email_email_address", false, "`email`,`email_address`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'email_from_addresses' collection: %w", err)
	}

	return nil
}

func createEmailToAddresses(app core.App) error {
	collection := core.NewCollection("base", "email_to_addresses")
	collection.Id = "ib_email_to_addresses"

	collection.ListRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "email",
			CollectionId: "ib_emails",
			MinSelect:    1,
			MaxSelect:    1,
			Required:     true,
		},
		&core.EmailField{
			Name:        "email_address",
			Presentable: true,
		},
		&core.TextField{
			Name:        "display_name",
			Presentable: true,
		},
	)

	collection.AddIndex("idx_ib_email_to_addresses_email_email_address", false, "`email`,`email_address`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'email_to_addresses' collection: %w", err)
	}

	return nil
}

func createEmailReplyToAddresses(app core.App) error {
	collection := core.NewCollection("base", "email_reply_to_addresses")
	collection.Id = "ib_email_reply_to_addresses"

	collection.ListRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "email",
			CollectionId: "ib_emails",
			MinSelect:    1,
			MaxSelect:    1,
			Required:     true,
		},
		&core.EmailField{
			Name:        "email_address",
			Presentable: true,
		},
		&core.TextField{
			Name:        "display_name",
			Presentable: true,
		},
	)

	collection.AddIndex("idx_ib_email_reply_to_addresses_email_email_address", false, "`email`,`email_address`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'email_reply_to_addresses' collection: %w", err)
	}

	return nil
}

func createEmailCcAddresses(app core.App) error {
	collection := core.NewCollection("base", "email_cc_addresses")
	collection.Id = "ib_email_cc_addresses"

	collection.ListRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "email",
			CollectionId: "ib_emails",
			MinSelect:    1,
			MaxSelect:    1,
			Required:     true,
		},
		&core.EmailField{
			Name:        "email_address",
			Presentable: true,
		},
		&core.TextField{
			Name:        "display_name",
			Presentable: true,
		},
	)

	collection.AddIndex("idx_ib_email_cc_addresses_email_email_address", false, "`email`,`email_address`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'email_cc_addresses' collection: %w", err)
	}

	return nil
}

func createEmailBccAddresses(app core.App) error {
	collection := core.NewCollection("base", "email_bcc_addresses")
	collection.Id = "ib_email_bcc_addresses"

	collection.ListRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "email",
			CollectionId: "ib_emails",
			MinSelect:    1,
			MaxSelect:    1,
			Required:     true,
		},
		&core.EmailField{
			Name:        "email_address",
			Presentable: true,
		},
		&core.TextField{
			Name:        "display_name",
			Presentable: true,
		},
	)

	collection.AddIndex("idx_ib_email_bcc_addresses_email_email_address", false, "`email`,`email_address`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'email_bcc_addresses' collection: %w", err)
	}

	return nil
}

func createEmailAttachments(app core.App) error {
	collection := core.NewCollection("base", "email_attachments")
	collection.Id = "ib_email_attachments"

	collection.ListRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.ViewRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.CreateRule = types.Pointer(`@request.auth.id != ""`)
	collection.UpdateRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")
	collection.DeleteRule = types.Pointer("email.smtp_account.created_by.id = @request.auth.id")

	collection.Fields.Add(
		&core.RelationField{
			Name:         "email",
			CollectionId: "ib_emails",
			MinSelect:    1,
			MaxSelect:    1,
			Required:     true,
		},
		&core.NumberField{
			Name:        "index",
			Presentable: true,
			Min:         types.Pointer(0.0),
			OnlyInt:     true,
		},
		&core.TextField{
			Name:        "name",
			Presentable: true,
		},
		&core.TextField{
			Name:        "mime_type",
			Presentable: true,
		},
		&core.FileField{
			Name:      "content",
			MaxSize:   maxFileSize,
			MaxSelect: 1,
		},
	)

	collection.AddIndex("idx_ib_email_attachments_email_index", false, "`email`,`index`", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create 'email_attachments' collection: %w", err)
	}

	return nil
}

func init() {
	m.Register(func(app core.App) error {

		if err := updateAppSettings(app); err != nil {
			return err
		}

		if err := updatePbUsersAuth(app); err != nil {
			return err
		}

		if err := createSmtpAccounts(app); err != nil {
			return err
		}

		if err := createEmails(app); err != nil {
			return err
		}

		if err := createEmailFlags(app); err != nil {
			return err
		}

		if err := createEmailFromAddresses(app); err != nil {
			return err
		}

		if err := createEmailToAddresses(app); err != nil {
			return err
		}

		if err := createEmailReplyToAddresses(app); err != nil {
			return err
		}

		if err := createEmailCcAddresses(app); err != nil {
			return err
		}

		if err := createEmailBccAddresses(app); err != nil {
			return err
		}

		if err := createEmailAttachments(app); err != nil {
			return err
		}

		return nil
	}, nil)
}
