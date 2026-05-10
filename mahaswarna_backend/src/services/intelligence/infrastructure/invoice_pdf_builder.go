package infrastructure

import (
	"bytes"
	"fmt"
	"time"

	"github.com/signintech/gopdf"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"github.com/mahaswarna/intelligence/domain"
)

// IST is the Indian Standard Time location (+05:30).
var IST = mustLoadLocation("Asia/Kolkata")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load timezone %s: %v", name, err))
	}
	return loc
}

// inrPrinter formats numbers with Indian lakh/crore grouping (e.g. ₹1,23,456.78).
var inrPrinter = message.NewPrinter(language.Hindi)

// formatINR returns an INR-formatted price string with Indian digit grouping.
func formatINR(amount float64) string {
	return inrPrinter.Sprintf("%.2f", amount)
}

// InvoicePDFBuilder generates GST-compliant PDF invoices using gopdf.
// The generated PDF is returned as []byte; it is NOT persisted (ADR-001).
type InvoicePDFBuilder struct{}

func NewInvoicePDFBuilder() *InvoicePDFBuilder {
	return &InvoicePDFBuilder{}
}

// InvoicePDFInput bundles all data needed for PDF rendering.
type InvoicePDFInput struct {
	Invoice    domain.Invoice
	ShopName   string
	ShopAddr   string
	ShopGST    string
	ShopPhone  string
	GoldRate   float64
	SilverRate float64
}

// Build renders and returns the invoice PDF bytes.
func (b *InvoicePDFBuilder) Build(in InvoicePDFInput) ([]byte, error) {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	// Use the built-in Helvetica core font.
	// Noto Sans Devanagari support requires embedding the TTF via //go:embed;
	// this is tracked as a future enhancement for regional language invoice printing.
	pdf.SetFont("Helvetica", "", 12)

	ist := in.Invoice.GeneratedAt.In(IST)

	// ── Header ────────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetXY(40, 40)
	pdf.Cell(nil, "TAX INVOICE")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetXY(40, 65)
	pdf.Cell(nil, in.ShopName)
	pdf.SetXY(40, 78)
	pdf.Cell(nil, in.ShopAddr)
	pdf.SetXY(40, 91)
	pdf.Cell(nil, fmt.Sprintf("GSTIN: %s  |  Phone: %s", in.ShopGST, in.ShopPhone))

	// Right side: Invoice metadata
	pdf.SetXY(380, 65)
	pdf.Cell(nil, fmt.Sprintf("Invoice #: %s", in.Invoice.ID.String()[:8]))
	pdf.SetXY(380, 78)
	pdf.Cell(nil, fmt.Sprintf("Date: %s", ist.Format("02 Jan 2006")))
	pdf.SetXY(380, 91)
	pdf.Cell(nil, fmt.Sprintf("Time: %s IST", ist.Format("15:04")))

	// ── Customer ──────────────────────────────────────────────────────────────
	pdf.SetXY(40, 115)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(nil, "Bill To:")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetXY(40, 128)
	pdf.Cell(nil, in.Invoice.CustomerName)
	if in.Invoice.CustomerPhone != nil {
		pdf.SetXY(40, 141)
		pdf.Cell(nil, fmt.Sprintf("Phone: %s", *in.Invoice.CustomerPhone))
	}

	// ── Rate info ─────────────────────────────────────────────────────────────
	pdf.SetXY(380, 115)
	pdf.SetFont("Helvetica", "", 9)
	pdf.Cell(nil, fmt.Sprintf("Gold: \u20b9%s/g  Silver: \u20b9%s/g",
		formatINR(in.GoldRate), formatINR(in.SilverRate)))
	pdf.SetXY(380, 128)
	pdf.Cell(nil, fmt.Sprintf("Rate source: %s", in.Invoice.RateSource))

	// ── Line items table header ───────────────────────────────────────────────
	tableY := 165.0
	pdf.SetXY(40, tableY)
	pdf.SetFont("Helvetica", "B", 9)
	drawRow(&pdf, tableY, []string{"Description", "Wt(g)", "Kt", "Making(\u20b9)", "Unit(\u20b9)", "Amt(\u20b9)", "GST(\u20b9)", "Net(\u20b9)"})
	tableY += 14

	// ── Line items ───────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "", 9)
	var totalNet float64
	var totalGST float64
	for _, item := range in.Invoice.Items {
		drawRow(&pdf, tableY, []string{
			item.Description,
			fmt.Sprintf("%.3f", item.WeightGrams),
			fmt.Sprintf("%dK", item.Karat),
			formatINR(item.MakingCharge),
			formatINR(item.UnitPrice),
			formatINR(item.TotalPrice),
			formatINR(item.GSTAmount),
			formatINR(item.NetAmount),
		})
		tableY += 14
		totalNet += item.NetAmount
		totalGST += item.GSTAmount
	}

	// ── Totals ───────────────────────────────────────────────────────────────
	tableY += 6
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetXY(380, tableY)
	pdf.Cell(nil, fmt.Sprintf("GST @ 3%%: \u20b9%s", formatINR(totalGST)))
	tableY += 14
	pdf.SetXY(380, tableY)
	pdf.Cell(nil, fmt.Sprintf("TOTAL: \u20b9%s", formatINR(totalNet)))

	// ── Payment mode ─────────────────────────────────────────────────────────
	tableY += 20
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(40, tableY)
	pdf.Cell(nil, fmt.Sprintf("Payment Mode: %s", in.Invoice.PaymentMode))

	if in.Invoice.Notes != nil && *in.Invoice.Notes != "" {
		tableY += 14
		pdf.SetXY(40, tableY)
		pdf.Cell(nil, fmt.Sprintf("Notes: %s", *in.Invoice.Notes))
	}

	// ── Footer ───────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetXY(40, 810)
	pdf.Cell(nil, "This is a computer-generated invoice. No signature required.")
	pdf.SetXY(40, 820)
	pdf.Cell(nil, "Generated by MahaSwarna \u2013 mahaswarna.com")

	var buf bytes.Buffer
	if err := pdf.Write(&buf); err != nil {
		return nil, fmt.Errorf("pdf write: %w", err)
	}
	return buf.Bytes(), nil
}

// drawRow writes a fixed-column row at y. Column widths are hard-coded for A4.
func drawRow(pdf *gopdf.GoPdf, y float64, cols []string) {
	xs := []float64{40, 160, 195, 225, 280, 340, 400, 455}
	for i, col := range cols {
		if i >= len(xs) {
			break
		}
		pdf.SetXY(xs[i], y)
		pdf.Cell(nil, col)
	}
}
