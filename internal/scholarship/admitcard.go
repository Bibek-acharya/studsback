package scholarship

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// AdmitCardData holds all the information needed to render the admit card.
type AdmitCardData struct {
	CandidateName    string
	DateOfBirth      string
	Gender           string
	RollNumber       string
	ExamCentre       string
	Stream           string
	PhotoURL         string
	ScholarshipTitle string
	Provider         string
	ExamDate         string
	ExamTime         string
	Shift            string
	SubjectName      string
}

const admitCardHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Project Shiksha Admit Card</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Poppins:wght@400;500;600;700;800&display=swap" rel="stylesheet">
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
        tailwind.config = {
            theme: {
                extend: {
                    fontFamily: { sans: ['"Poppins"', 'sans-serif'] },
                    colors: { tableHeader: '#e0f2f1', instructionText: '#0066cc' }
                }
            }
        }
    </script>
    <style>
        body { background: white; margin: 0; padding: 0; }
        .a4-paper { width: 210mm; min-height: 297mm; background: white; margin: 0 auto; position: relative; box-sizing: border-box; display: flex; flex-direction: column; padding: 10mm 12mm; }
        * { -webkit-print-color-adjust: exact !important; print-color-adjust: exact !important; }
    </style>
</head>
<body>
<div id="admit-card-content" class="a4-paper">
    <div class="absolute inset-0 flex items-center justify-center pointer-events-none z-0">
        <img src="https://projectshiksha.hundredgroupnepal.org/images/shiks.jpg" alt="" class="w-[65%] object-contain opacity-5" onerror="this.style.display='none'">
    </div>
    <div class="relative z-10 flex flex-col h-full">
        <div class="flex items-center justify-between pb-2 w-full">
            <div class="w-32 h-32 shrink-0 flex items-center justify-center">
                <img src="https://projectshiksha.hundredgroupnepal.org/images/shiks.jpg" alt="Logo" class="max-w-full max-h-full object-contain" onerror="this.src='https://via.placeholder.com/150?text=Logo'">
            </div>
            <div class="text-center flex flex-col justify-center flex-1 px-2">
                <h1 class="text-[26px] font-bold tracking-wide text-black uppercase mb-1">PROJECT SHIKSHA</h1>
                <h2 class="text-[13px] font-semibold text-gray-800 uppercase tracking-wider mb-2">ENTRANCE EXAMINATION FOR 2083 BATCH</h2>
                <h3 class="text-[14px] font-bold text-black uppercase tracking-widest">Admit Card</h3>
            </div>
            <div class="w-32 shrink-0 flex items-center justify-end">
                <div class="w-20 h-20 border border-gray-400 p-1 bg-white">
                    <img src="https://upload.wikimedia.org/wikipedia/commons/d/d0/QR_code_for_mobile_English_Wikipedia.svg" alt="QR" class="w-full h-full object-contain">
                </div>
            </div>
        </div>
        <div class="w-full border-b-[1.5px] border-gray-400 mb-5 mt-3"></div>
        <div class="flex justify-between mb-5">
            <div class="flex-1 grid grid-cols-[140px_10px_1fr] gap-y-2 text-[12.5px] text-gray-800 items-center">
                <div class="font-semibold">Candidate's Name</div><div>:</div><div class="font-semibold text-black text-[14px]">{{.CandidateName}}</div>
                <div class="font-semibold">Date of Birth</div><div>:</div><div class="font-semibold text-black">{{.DateOfBirth}}</div>
                <div class="font-semibold">Gender</div><div>:</div><div class="font-semibold text-black">{{.Gender}}</div>
                <div class="font-semibold">Roll Number</div><div>:</div><div class="font-semibold text-black">{{.RollNumber}}</div>
                <div class="font-semibold">Exam Centre Name</div><div>:</div><div class="font-semibold text-black">{{.ExamCentre}}</div>
                <div class="font-semibold">Stream</div><div>:</div><div class="font-semibold text-black">{{.Stream}}</div>
            </div>
            <div class="w-[120px] ml-4 shrink-0 flex flex-col items-center">
                <div class="w-full h-[140px] border border-gray-400 mb-1 p-0.5 bg-white">
                    {{if .PhotoURL}}<img src="{{.PhotoURL}}" alt="Photo" class="w-full h-full object-cover">
                    {{else}}<div class="w-full h-full bg-gray-100 flex items-center justify-center text-xs text-gray-400">Photo</div>{{end}}
                </div>
                <div class="w-full px-1 text-center mt-5">
                    <div class="w-full border-b-[1.5px] border-dashed border-gray-800 h-2 mb-1.5"></div>
                    <p class="text-[9px] font-semibold text-gray-800 uppercase tracking-widest">Student Signature</p>
                </div>
            </div>
        </div>
        <div class="mb-8">
            <div class="text-[12px] font-bold text-black mb-1.5 pl-1">Details of Examination Subject (With scheduled exam program)</div>
            <table class="w-full border-collapse border border-gray-400 text-[11.5px] text-center">
                <thead class="bg-tableHeader text-gray-800">
                    <tr>
                        <th class="border border-gray-400 py-1.5 px-2 font-semibold">Subject Name</th>
                        <th class="border border-gray-400 py-1.5 px-2 font-semibold">Exam Date</th>
                        <th class="border border-gray-400 py-1.5 px-2 font-semibold">Shift</th>
                        <th class="border border-gray-400 py-1.5 px-2 font-semibold">Exam Time</th>
                    </tr>
                </thead>
                <tbody class="text-black font-medium">
                    <tr>
                        <td class="border border-gray-400 py-3 px-2 text-left pl-4 font-bold">{{.SubjectName}}</td>
                        <td class="border border-gray-400 py-3 px-2">{{.ExamDate}}</td>
                        <td class="border border-gray-400 py-3 px-2">{{.Shift}}</td>
                        <td class="border border-gray-400 py-3 px-2">{{.ExamTime}}</td>
                    </tr>
                    <tr>
                        <td class="border border-gray-400 py-3 px-2"></td>
                        <td class="border border-gray-400 py-3 px-2"></td>
                        <td class="border border-gray-400 py-3 px-2"></td>
                        <td class="border border-gray-400 py-3 px-2"></td>
                    </tr>
                </tbody>
            </table>
        </div>
        <div class="flex justify-between items-end px-4 mt-auto mb-10">
            <div class="flex flex-col items-center">
                <div class="w-48 border-b-[2px] border-dotted border-gray-800 mb-2 mt-12"></div>
                <span class="text-[11.5px] font-bold text-black text-center">Authorized Seal &amp; Signature<br><span class="font-normal text-[10px]">(Head of Institution)</span></span>
            </div>
            <div class="flex flex-col items-center">
                <img src="https://upload.wikimedia.org/wikipedia/commons/f/fa/Signature_of_John_Hancock.svg" class="h-8 mb-1 opacity-80" alt="Sig" onerror="this.style.display='none'">
                <div class="w-48 border-b-[2px] border-dotted border-gray-800 mb-2"></div>
                <span class="text-[11.5px] font-bold text-black">Controller of Examination</span>
            </div>
        </div>
        <div class="border-t-[1.5px] border-dashed border-gray-400 pt-4 pb-2 mt-4">
            <div class="text-center mb-3">
                <h3 class="text-[13px] font-bold text-instructionText uppercase tracking-wide">Important Instructions for Candidates</h3>
            </div>
            <table class="w-full text-left">
                <tbody class="text-[11px] text-gray-800 leading-[1.6]">
                    <tr><td class="py-1 pr-2 align-top text-black w-5 text-right">1.</td><td class="py-1">Candidates must bring this Original Admit Card along with a valid Original Photo ID proof (Citizenship/School ID) to the examination centre.</td></tr>
                    <tr><td class="py-1 pr-2 align-top text-black text-right">2.</td><td class="py-1">Candidates will be permitted to sit in their designated seats only. Latecomers (more than half an hour late) may not be allowed to take the exam.</td></tr>
                    <tr><td class="py-1 pr-2 align-top text-black text-right">3.</td><td class="py-1">Electronic gadgets, mobile phones, smartwatches, and programmable calculators are strictly prohibited inside the hall.</td></tr>
                    <tr><td class="py-1 pr-2 align-top text-black text-right">4.</td><td class="py-1">Use only Black/Blue ballpoint pen. Use of pencils for marking answers is strictly prohibited unless specified.</td></tr>
                    <tr><td class="py-1 pr-2 align-top text-black text-right">5.</td><td class="py-1">Impersonation or any form of malpractice will lead to immediate disqualification and legal action.</td></tr>
                    <tr><td class="py-1 pr-2 align-top text-black text-right">6.</td><td class="py-1">Any discrepancy in the admit card must be reported to the examination authority immediately for correction.</td></tr>
                    <tr><td class="py-1 pr-2 align-top text-black text-right">7.</td><td class="py-1">Candidates must preserve this admit card securely until the final admission process is completed.</td></tr>
                    <tr><td class="py-1 pr-2 align-top text-black text-right">8.</td><td class="py-1">Do not write anything on the front or back of this admit card. Rough work must be done on the provided sheet.</td></tr>
                </tbody>
            </table>
        </div>
    </div>
</div>
</body>
</html>`

// PhotoToBase64 reads an uploaded photo from the local filesystem and returns a base64 data URL.
// The path should be a relative upload path like "/uploads/scholarship/photos/12345.jpg".
func PhotoToBase64(photoPath string) string {
	if photoPath == "" {
		return ""
	}
	// Strip leading slash and use local path
	localPath := photoPath
	if localPath[0] == '/' {
		localPath = "." + localPath
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return ""
	}
	mimeType := "image/jpeg"
	if len(data) > 4 {
		if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
			mimeType = "image/png"
		} else if data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
			mimeType = "image/gif"
		} else if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
			mimeType = "image/webp"
		} else if data[0] == '<' {
			mimeType = "image/svg+xml"
		}
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

// GenerateAdmitCardPDF renders the admit card HTML with the given data and returns a PDF as bytes.
func GenerateAdmitCardPDF(data AdmitCardData) ([]byte, error) {
	if data.SubjectName == "" {
		data.SubjectName = data.Stream
	}
	if data.Shift == "" {
		data.Shift = "1st"
	}
	if data.RollNumber == "" {
		data.RollNumber = fmt.Sprintf("PS-%d", time.Now().UnixNano()%1000000000)
	}

	tmpl, err := template.New("admitcard").Parse(admitCardHTMLTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse admit card template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to render admit card template: %w", err)
	}

	// Embed HTML as a base64 data URL so chromedp can navigate to it without temp files
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	dataURL := "data:text/html;base64," + encoded

	// Find the chromium/chrome executable
	chromiumPath := ""
	for _, candidate := range []string{
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
	} {
		if _, err := os.Stat(candidate); err == nil {
			chromiumPath = candidate
			break
		}
	}

	var allocCtx context.Context
	var allocCancel context.CancelFunc
	if chromiumPath != "" {
		allocCtx, allocCancel = chromedp.NewExecAllocator(
			context.Background(),
			append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.ExecPath(chromiumPath),
				chromedp.Flag("no-sandbox", true),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("headless", true),
			)...,
		)
	} else {
		allocCtx, allocCancel = chromedp.NewExecAllocator(
			context.Background(),
			append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("no-sandbox", true),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("headless", true),
			)...,
		)
	}
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var pdfBuf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(dataURL),
		chromedp.WaitReady("body"),
		// Wait for Tailwind CDN to render
		chromedp.Sleep(3*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			result, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(8.27).   // A4 in inches
				WithPaperHeight(11.69). // A4 in inches
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				Do(ctx)
			if err != nil {
				return err
			}
			pdfBuf = result
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return pdfBuf, nil
}
