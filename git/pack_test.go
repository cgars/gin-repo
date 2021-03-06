package git

import (
	"bytes"
	"crypto/sha1"
	"io"
	"testing"
)

func TestPackBasic(t *testing.T) {

	repo, err := DiscoverRepository()

	if err != nil {
		t.Skip("[W] Not in git directory. Skipping test")
	}

	indicies := repo.loadPackIndices()

	if len(indicies) < 1 {
		t.Skip("[W] No pack files found. Skipping test")
	}

	for _, f := range indicies {
		t.Logf("testing pack: %s", f)
		idx, err := PackIndexOpen(f)
		if err != nil {
			t.Fatalf("could not open pack index: %v", err)
		}

		//TODO: we should leave index files open,
		defer idx.Close()

		data, err := idx.OpenPackFile()

		if err != nil {
			t.Fatalf("could not open pack file: %v", err)
		}

		count := 0
		for i := byte(0); i < 255; i++ {
			s, e := idx.FO.Bounds(i)
			for k := s; k < e; k++ {
				var oid SHA1

				err := idx.ReadSHA1(&oid, k)
				if err != nil {
					t.Fatalf("could not read sha1 at pos %d: %v", k, err)
				}

				//t.Logf("\t obj %s", oid)

				//we use FindOffset, not ReadOffset, to test the
				//search functionality
				off, err := idx.FindOffset(oid)

				if err != nil {
					t.Fatalf("could not find sha1 (%s) that was in the index: %v", oid, err)
				}

				o2, err := idx.ReadOffset(k)

				if err != nil {
					t.Fatalf("could not read offset at %d that was in the index: %v", k, err)
				}

				if o2 != off {
					t.Fatalf("offset returned by FindOffset differs from ReadOffset")
				}

				obj, err := data.OpenObject(off)

				if err != nil {
					t.Fatalf("could not open object (%s) at %d: %v", oid, k, err)
				}

				if IsDeltaObject(obj.Type()) {
					//t.Logf("checking delta obj: %q", oid)
					delta := obj.(*Delta)
					chain, err := buildDeltaChain(delta, repo)

					if err != nil {
						t.Fatalf("building delta chain failed for %q: %v", oid, err)
					}

					obj, err = chain.resolve()

					if err != nil {
						t.Fatalf("resolving delta chain failed for %q: %v", oid, err)
					}
				}

				var b bytes.Buffer
				h := sha1.New()
				mw := io.MultiWriter(h, &b)

				_, err = obj.WriteTo(mw)
				if err != nil {
					t.Fatalf("Object.WriteTo(%q) => failed!: %v ", oid, err)
				}

				hid := h.Sum(nil)
				var cid SHA1
				copy(cid[:], hid)

				if cid != oid {
					t.Logf("[E] object proof:\n%s---EOF---\n", b.String())
					t.Fatalf("sha1(%s) => %q expected %q", obj.Type(), cid, oid)
				}

				count++
			}
		}

		t.Logf("tested %d objects in pack", count)

		onf, err := ParseSHA1("0000000000000000000000000000000000000000")
		if err != nil {
			t.Fatalf("could not parse all-zero sha1: %v", err)
		}

		off, err := idx.FindOffset(onf)

		if err == nil {
			t.Fatalf("found all-zero sha1 @: %d", off)
		}

		onf, err = ParseSHA1("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
		if err != nil {
			t.Fatalf("could not parse all-0xF sha1: %v", err)
		}

		off, err = idx.FindOffset(onf)
		if err == nil {
			t.Fatalf("found all-0xF sha1 @: %d", off)
		}
	}
}
