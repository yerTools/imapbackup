package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/BrianLeishman/go-imap"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	"github.com/yerTools/imapbackup/src/go/database"
)

const (
	syncBatchSize = 50
)

func main() {
	// loosely check if it was executed using "go run"
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())

	app := pocketbase.NewWithConfig(
		pocketbase.Config{
			DefaultDev: false,
		},
	)
	app.RootCmd.Version = Version
	app.RootCmd.Use = "imapbackup"
	app.RootCmd.Short = ""

	database.Init(app, isGoRun)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// app.Cron().MustAdd("sync mails", "0 0 31 2 1", syncMails(app))
		app.Cron().MustAdd("sync mails", "* * * * *", syncMails(app))

		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func syncMails(app *pocketbase.PocketBase) func() {
	return func() {
		log.Printf("syncing mails with batch size %d ...\n", syncBatchSize)

		imap.Verbose = false
		imap.RetryCount = 3

		ib_emails, err := app.FindCollectionByNameOrId("ib_emails")
		if err != nil {
			log.Printf("failed to find 'ib_emails' collection: %v\n", err)
			return
		}

		ib_email_flags, err := app.FindCollectionByNameOrId("ib_email_flags")
		if err != nil {
			log.Printf("failed to find 'ib_email_flags' collection: %v\n", err)
			return
		}

		ib_email_from_addresses, err := app.FindCollectionByNameOrId("ib_email_from_addresses")
		if err != nil {
			log.Printf("failed to find 'ib_email_from_addresses' collection: %v\n", err)
			return
		}

		ib_email_to_addresses, err := app.FindCollectionByNameOrId("ib_email_to_addresses")
		if err != nil {
			log.Printf("failed to find 'ib_email_to_addresses' collection: %v\n", err)
			return
		}

		ib_email_reply_to_addresses, err := app.FindCollectionByNameOrId("ib_email_reply_to_addresses")
		if err != nil {
			log.Printf("failed to find 'ib_email_reply_to_addresses' collection: %v\n", err)
			return
		}

		ib_email_cc_addresses, err := app.FindCollectionByNameOrId("ib_email_cc_addresses")
		if err != nil {
			log.Printf("failed to find 'ib_email_cc_addresses' collection: %v\n", err)
			return
		}

		ib_email_bcc_addresses, err := app.FindCollectionByNameOrId("ib_email_bcc_addresses")
		if err != nil {
			log.Printf("failed to find 'ib_email_bcc_addresses' collection: %v\n", err)
			return
		}

		ib_email_attachments, err := app.FindCollectionByNameOrId("ib_email_attachments")
		if err != nil {
			log.Printf("failed to find 'ib_email_attachments' collection: %v\n", err)
			return
		}

		smtpAccounts, err := app.FindAllRecords("ib_smtp_accounts")
		if err != nil {
			log.Printf("failed to find SMTP accounts: %v\n", err)
			return
		}

		log.Printf("found %d SMTP account(s)\n", len(smtpAccounts))

		for _, smtpAccount := range smtpAccounts {
			log.Printf("syncing user %s on %s with port %d ...\n", smtpAccount.GetString("username"), smtpAccount.GetString("host"), smtpAccount.GetInt("port"))
			func() {
				im, err := imap.New(smtpAccount.GetString("username"), smtpAccount.GetString("password"), smtpAccount.GetString("host"), smtpAccount.GetInt("port"))
				if err != nil {
					log.Printf("failed to connect: %v\n", err)
					return
				}
				defer im.Close()

				folders, err := im.GetFolders()
				if err != nil {
					log.Printf("failed to get folders: %v\n", err)
					return
				}

				log.Printf("found %d folder(s)\n", len(folders))

				for _, folder := range folders {
					log.Printf("syncing folder %s ...\n", folder)

					err = im.SelectFolder(folder)
					if err != nil {
						log.Printf("failed to select folder: %v\n", err)
						return
					}

					uids, err := im.GetUIDs("ALL")
					if err != nil {
						log.Printf("failed to get UIDs: %v\n", err)
						return
					}
					log.Printf("found %d email(s)\n", len(uids))

					syncMails := make([]int, 0, len(uids))

					for i := 0; i < len(uids); i += syncBatchSize {
						batchSliceEnd := i + syncBatchSize
						if batchSliceEnd > len(uids) {
							batchSliceEnd = len(uids)
						}
						uidsBatch := uids[i:batchSliceEnd]

						emailOverview, err := im.GetOverviews(uidsBatch...)
						if err != nil {
							log.Printf("failed to get email overviews: %v\n", err)
							return
						}

						for _, overview := range emailOverview {
							existingMails, err := app.FindRecordsByFilter(
								"ib_emails",
								`smtp_account.id = {:smtp_account_id} &&
								message_id = {:message_id} &&
								size = {:size} &&
								subject = {:subject}`,
								"",
								0,
								0,
								dbx.Params{
									"smtp_account_id": smtpAccount.Id,
									"message_id":      overview.MessageID,
									"size":            overview.Size,
									"subject":         overview.Subject,
								},
							)
							if err != nil {
								log.Printf("failed to find existing mails: %v\n", err)
								return
							}

							existingMailsCopy := make([]*core.Record, 0, len(existingMails))
							for _, existingMail := range existingMails {
								received := existingMail.GetDateTime("received")
								sent := existingMail.GetDateTime("sent")
								if overview.Received.Unix() == received.Unix() && overview.Sent.Unix() == sent.Unix() {
									existingMailsCopy = append(existingMailsCopy, existingMail)
									continue
								}

								log.Println("time mismatch")
							}
							existingMails = existingMailsCopy

							if len(existingMails) == 0 {
								syncMails = append(syncMails, overview.UID)
								continue
							}

							for _, existingMail := range existingMails {
								if existingMail.GetString("folder") == folder {
									continue
								}
								existingMail.Set("folder", folder)
								err := app.Save(existingMail)
								if err != nil {
									log.Printf("failed to save existing email: %v\n", err)
									return
								}
								log.Printf("moved email to folder %s\n", folder)
							}

						}
					}

					log.Printf("found %d email(s) to sync\n", len(syncMails))

					for i := 0; i < len(syncMails); i += syncBatchSize {
						batchSliceEnd := i + syncBatchSize
						if batchSliceEnd > len(syncMails) {
							batchSliceEnd = len(syncMails)
						}
						uidsBatch := syncMails[i:batchSliceEnd]

						emails, err := im.GetEmails(uidsBatch...)
						if err != nil {
							log.Printf("failed to get emails: %v\n", err)
							continue
						}

						for _, email := range emails {
							err = app.RunInTransaction(func(txApp core.App) error {
								email_record := core.NewRecord(ib_emails)

								email_record.Set("smtp_account", smtpAccount.Id)
								email_record.Set("folder", folder)
								email_record.Set("received", email.Received)
								email_record.Set("sent", email.Sent)
								email_record.Set("size", email.Size)
								email_record.Set("subject", email.Subject)
								email_record.Set("uid", email.UID)
								email_record.Set("message_id", email.MessageID)
								email_record.Set("text", email.Text)
								email_record.Set("html", email.HTML)

								err := txApp.Save(email_record)
								if err != nil {
									return fmt.Errorf("failed to save email record: %w", err)
								}

								for index, flag := range email.Flags {
									email_flag := core.NewRecord(ib_email_flags)
									email_flag.Set("email", email_record.Id)
									email_flag.Set("index", index)
									email_flag.Set("flag", flag)

									err := txApp.Save(email_flag)
									if err != nil {
										return fmt.Errorf("failed to save email flag: %w", err)
									}
								}

								for email_address, display_name := range email.From {
									email_from_address := core.NewRecord(ib_email_from_addresses)
									email_from_address.Set("email", email_record.Id)
									email_from_address.Set("email_address", email_address)
									email_from_address.Set("display_name", display_name)

									err := txApp.Save(email_from_address)
									if err != nil {
										return fmt.Errorf("failed to save email from address: %w", err)
									}
								}

								for email_address, display_name := range email.To {
									email_to_address := core.NewRecord(ib_email_to_addresses)
									email_to_address.Set("email", email_record.Id)
									email_to_address.Set("email_address", email_address)
									email_to_address.Set("display_name", display_name)

									err := txApp.Save(email_to_address)
									if err != nil {
										return fmt.Errorf("failed to save email to address: %w", err)
									}
								}

								for email_address, display_name := range email.ReplyTo {
									email_reply_to_address := core.NewRecord(ib_email_reply_to_addresses)
									email_reply_to_address.Set("email", email_record.Id)
									email_reply_to_address.Set("email_address", email_address)
									email_reply_to_address.Set("display_name", display_name)

									err := txApp.Save(email_reply_to_address)
									if err != nil {
										return fmt.Errorf("failed to save email reply to address: %w", err)
									}
								}

								for email_address, display_name := range email.CC {
									email_cc_address := core.NewRecord(ib_email_cc_addresses)
									email_cc_address.Set("email", email_record.Id)
									email_cc_address.Set("email_address", email_address)
									email_cc_address.Set("display_name", display_name)

									err := txApp.Save(email_cc_address)
									if err != nil {
										return fmt.Errorf("failed to save email cc address: %w", err)
									}
								}

								for email_address, display_name := range email.BCC {
									email_bcc_address := core.NewRecord(ib_email_bcc_addresses)
									email_bcc_address.Set("email", email_record.Id)
									email_bcc_address.Set("email_address", email_address)
									email_bcc_address.Set("display_name", display_name)

									err := txApp.Save(email_bcc_address)
									if err != nil {
										return fmt.Errorf("failed to save email bcc address: %w", err)
									}
								}

								for index, attachment := range email.Attachments {
									email_attachment := core.NewRecord(ib_email_attachments)
									email_attachment.Set("email", email_record.Id)
									email_attachment.Set("index", index)
									email_attachment.Set("name", attachment.Name)
									email_attachment.Set("mime_type", attachment.MimeType)

									content_file, err := filesystem.NewFileFromBytes(attachment.Content, attachment.Name)
									if err != nil {
										return fmt.Errorf("failed to create content file: %w", err)
									}

									email_attachment.Set("content", content_file)

									err = txApp.Save(email_attachment)
									if err != nil {
										return fmt.Errorf("failed to save email attachment: %w", err)
									}
								}

								return nil
							})
							if err != nil {
								log.Printf("failed to sync email: %v\n", err)
								continue
							}
						}

						log.Printf("synced %d/%d email(s)\n", batchSliceEnd, len(syncMails))
					}
				}
			}()
		}

		log.Println("syncing done")
	}
}
